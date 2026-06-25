package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

var (
	env          = os.Getenv("ENV")
	webhookCache = make(map[string]WebhookCache)
	cacheMutex   sync.RWMutex
	httpClient   = &http.Client{Timeout: 250 * time.Millisecond}
	ssmClient    *ssm.Client
	awsOnce      sync.Once
	awsInitErr   error
	dbPool       *sql.DB
	dbOnce       sync.Once
	dbInitErr    error
	orgIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

const (
	maxRetries          = 3
	retryBackoff        = 2 * time.Second
	cacheExpiration     = 10 * time.Minute
	dbMaxOpenConns      = 5
	dbMaxIdleConns      = 2
	dbConnMaxLifetime   = 5 * time.Minute
	slackHooksURLPrefix = "https://hooks.slack.com/"
)

type WebhookCache struct {
	Updated  time.Time
	Webhooks []WebhookRecord
}

type WebhookRecord struct {
	APIURL    string
	EventName string
	DatasetID int
}

func initAWS(ctx context.Context) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		awsInitErr = fmt.Errorf("unable to load SDK config: %w", err)
		return
	}
	ssmClient = ssm.NewFromConfig(cfg)
}

func initDB(ctx context.Context) error {
	dbname, dberr := getSSMParam(ctx, fmt.Sprintf("/%s/integration-service/integrations-postgres-db", env), false)
	if dberr != nil {
		return fmt.Errorf("failed to get DB name: %w", dberr)
	}
	dbusername, usererr := getSSMParam(ctx, fmt.Sprintf("/%s/integration-service/integrations-postgres-user", env), false)
	if usererr != nil {
		return fmt.Errorf("failed to get DB username: %w", usererr)
	}
	dbpassword, pwerr := getSSMParam(ctx, fmt.Sprintf("/%s/integration-service/integrations-postgres-password", env), true)
	if pwerr != nil {
		return fmt.Errorf("failed to get DB password: %w", pwerr)
	}
	dbhostname, hosterr := getSSMParam(ctx, fmt.Sprintf("/%s/integration-service/integrations-postgres-host", env), false)
	if hosterr != nil {
		return fmt.Errorf("failed to get DB hostname: %w", hosterr)
	}

	connectionStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=require",
		dbhostname, dbusername, dbpassword, dbname)

	db, err := sql.Open("postgres", connectionStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(dbMaxOpenConns)
	db.SetMaxIdleConns(dbMaxIdleConns)
	db.SetConnMaxLifetime(dbConnMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	dbPool = db
	return nil
}

func init() {
	log.Println("Lambda CONSTRUCTOR invoked at", time.Now())
}

func getSSMParam(ctx context.Context, name string, decrypt bool) (string, error) {
	if awsInitErr != nil {
		return "", awsInitErr
	}

	if ssmClient == nil {
		return "", fmt.Errorf("uninitialized SSM client")
	}

	input := &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: &decrypt,
	}

	output, err := ssmClient.GetParameter(ctx, input)
	if err != nil {
		return "", fmt.Errorf("unable to fetch SSM parameter %s: %w", name, err)
	}
	return *output.Parameter.Value, nil
}

func ensureDB(ctx context.Context) error {
	dbOnce.Do(func() {
		dbInitErr = initDB(ctx)
	})
	return dbInitErr
}

func query(ctx context.Context, command string) ([]WebhookRecord, error) {
	if dbPool == nil {
		return nil, fmt.Errorf("database pool not initialized")
	}

	rows, err := dbPool.QueryContext(ctx, command)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v\n", err)
		}
	}()

	var res []WebhookRecord

	for rows.Next() {
		var r WebhookRecord
		err := rows.Scan(&r.APIURL, &r.EventName, &r.DatasetID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		res = append(res, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return res, nil
}

func refreshWebhookCache(ctx context.Context, orgID string) {
	if !orgIDPattern.MatchString(orgID) {
		log.Printf("invalid org id: %q\n", orgID)
		return
	}

	command := fmt.Sprintf(`
		SELECT wh.api_url, wet.event_name, wi.dataset_id
		FROM "%s".webhooks AS wh
		INNER JOIN "%s".webhook_event_subscriptions as wes ON wh.id=wes.webhook_id
		INNER JOIN "%s".dataset_integrations as wi ON wh.id=wi.webhook_id
		INNER JOIN "%s".webhook_event_types as wet ON wes.webhook_event_type_id=wet.id
	`, orgID, orgID, orgID, orgID)

	results, err := query(ctx, command)
	if err != nil {
		log.Printf("SQL Query error for org %s: %v\n", orgID, err)
		return
	}

	cacheMutex.Lock()
	webhookCache[orgID] = WebhookCache{
		Updated:  time.Now(),
		Webhooks: results,
	}
	cacheMutex.Unlock()
	log.Printf("Refreshed webhook cache for org %s with %d webhooks\n", orgID, len(results))
}

type EventMessage struct {
	OrgID    string `json:"organizationId"`
	DataID   int    `json:"datasetId"`
	Category string `json:"eventCategory"`
	Type     string `json:"eventType"`
}

func mapEvents(events map[string]interface{}) (map[string][]EventMessage, bool, error) {
	mapped := make(map[string][]EventMessage)
	forceRefresh := false

	records, ok := events["Records"].([]interface{})
	if !ok {
		return nil, false, fmt.Errorf("invalid event format: records field missing or wrong type")
	}
	for _, r := range records {
		rec, ok := r.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("record not an object")
		}
		rawBody, ok := rec["body"]
		if !ok {
			return nil, false, fmt.Errorf("record.body missing")
		}
		body, ok := rawBody.(string)
		if !ok {
			return nil, false, fmt.Errorf("record.body not a string")
		}

		var bodyJSON map[string]interface{}
		err := json.Unmarshal([]byte(body), &bodyJSON)
		if err != nil {
			return nil, false, err
		}
		msgStr, ok := bodyJSON["Message"].(string)
		if !ok {
			return nil, false, fmt.Errorf("record.body.Message not a string")
		}
		var msg EventMessage
		err = json.Unmarshal([]byte(msgStr), &msg)
		if err != nil {
			return nil, false, err
		}

		mapped[msg.OrgID] = append(mapped[msg.OrgID], msg)

		if msg.Type == "CREATE_DATASET" {
			forceRefresh = true
		}
	}

	return mapped, forceRefresh, nil
}

type WebhookMessage struct {
	Messages []EventMessage
	URLs     []string
}

func buildWebhookLookup(webhooks []WebhookRecord) map[string][]string {
	lookup := make(map[string][]string)
	for _, w := range webhooks {
		key := fmt.Sprintf("%d:%s", w.DatasetID, w.EventName)
		lookup[key] = append(lookup[key], w.APIURL)
	}
	return lookup
}

func mapWebhookMessages(ctx context.Context, mapped map[string][]EventMessage, forceRefresh bool) map[string]WebhookMessage {
	result := make(map[string]WebhookMessage)

	for orgID, events := range mapped {
		cacheMutex.RLock()
		cacheEntry, exists := webhookCache[orgID]
		cacheMutex.RUnlock()

		if forceRefresh || !exists || time.Since(cacheEntry.Updated) > cacheExpiration {
			refreshWebhookCache(ctx, orgID)
			cacheMutex.RLock()
			cacheEntry, exists = webhookCache[orgID]
			cacheMutex.RUnlock()
		}

		if !exists || len(cacheEntry.Webhooks) == 0 {
			log.Printf("No webhooks found for org %s\n", orgID)
			continue
		}

		webhookLookup := buildWebhookLookup(cacheEntry.Webhooks)

		for _, evt := range events {
			key := fmt.Sprintf("%d:%s", evt.DataID, evt.Category)

			entry := result[key]
			entry.Messages = append(entry.Messages, evt)

			if urls, ok := webhookLookup[key]; ok {
				entry.URLs = append(entry.URLs, urls...)
			}

			result[key] = entry
		}
	}

	return result
}

func webhookBodyParser(url string, msg EventMessage) ([]byte, error) {
	if strings.HasPrefix(url, slackHooksURLPrefix) {
		wrap := map[string]string{
			"text": string(mustJSON(msg)),
		}
		return mustJSON(wrap), nil
	}
	return mustJSON(msg), nil
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("JSON marshaling error: %v\n", err)
		return []byte("{}")
	}
	return b
}

func sendWebhookWithRetry(url string, body []byte) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		if err != nil {
			log.Printf("Failed to create request for %s (attempt %d): %v", url, attempt, err)
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Error closing response body: %v", err)
			}
			return nil
		}

		if resp != nil {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Error closing response body: %v", err)
			}
		}

		lastErr = err
		if attempt < maxRetries {
			jitter := time.Duration(rand.Int63n(1000)) * time.Millisecond
			backoffDuration := (retryBackoff * time.Duration(attempt)) + jitter
			log.Printf("Attempt %d failed for %s (status: %v): %v. Retrying in %v", attempt, url, resp, err, backoffDuration)
			time.Sleep(backoffDuration)
		}
	}

	return fmt.Errorf("failed to deliver message to %s after %d attempts: %w", url, maxRetries, lastErr)
}

func broadcastMessages(messages map[string]WebhookMessage) {
	for _, record := range messages {
		urlSet := make(map[string]bool)
		for _, url := range record.URLs {
			urlSet[url] = true
		}

		for url := range urlSet {
			for _, msg := range record.Messages {
				body, err := webhookBodyParser(url, msg)
				if err != nil {
					log.Printf("Failed to parse webhook body for %s: %v", url, err)
					continue
				}

				if err := sendWebhookWithRetry(url, body); err != nil {
					log.Printf("Failed to send webhook: %v", err)
					continue
				}
			}
		}
	}
}

func handler(ctx context.Context, events map[string]interface{}) (map[string]interface{}, error) {
	awsOnce.Do(func() {
		initAWS(ctx)
	})

	if err := ensureDB(ctx); err != nil {
		return nil, fmt.Errorf("database initialization failed: %w", err)
	}

	log.Println("Lambda handler invoked at", time.Now())

	mappedEvents, forceRefresh, err := mapEvents(events)
	if err != nil {
		return nil, fmt.Errorf("failed to map events: %w", err)
	}

	webhookMessages := mapWebhookMessages(ctx, mappedEvents, forceRefresh)
	broadcastMessages(webhookMessages)

	return events, nil
}

func main() {
	lambda.Start(handler)
}
