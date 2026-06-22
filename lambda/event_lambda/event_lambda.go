package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"net/http"
)

var (
	env           = os.Getenv("ENV")
	webhookCache  = make(map[string]WebhookCache)
	cacheMutex    sync.Mutex
	httpClient    = &http.Client{Timeout: 250 * time.Millisecond}
	ssmClient     *ssm.Client
)

type WebhookCache struct {
	LastUpdated time.Time
	Webhooks    []WebhookRecord
}

type WebhookRecord struct {
	APIURL    string
	EventName string
	DatasetID int
}

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}
	ssmClient = ssm.NewFromConfig(cfg)
	log.Println("Lambda CONSTRUCTOR invoked at", time.Now())
}

func getSSMParam(name string, decrypt bool) string {
	input := &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: &decrypt,
	}

	out, err := ssmClient.GetParameter(context.TODO(), input)
	if err != nil {
		log.Fatalf("Error fetching parameter %s: %v", name, err)
	}
	return *out.Parameter.Value
}

func connectDB() (*sql.DB, error) {
	dbname := getSSMParam(fmt.Sprintf("/%s/integration-service/integrations-postgres-db", env), false)
	dbuser := getSSMParam(fmt.Sprintf("/%s/integration-service/integrations-postgres-user", env), false)
	dbpass := getSSMParam(fmt.Sprintf("/%s/integration-service/integrations-postgres-password", env), true)
	dbhost := getSSMParam(fmt.Sprintf("/%s/integration-service/integrations-postgres-host", env), false)

	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		dbhost, dbuser, dbpass, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func query(db *sql.DB, command string) ([]WebhookRecord, error) {
	rows, err := db.Query(command)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []WebhookRecord

	for rows.Next() {
		var r WebhookRecord
		err := rows.Scan(&r.APIURL, &r.EventName, &r.DatasetID)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, nil
}

func refreshWebhookCache(orgID string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	db, err := connectDB()
	if err != nil {
		log.Println("Connection error:", err)
		return
	}
	defer db.Close()

	command := fmt.Sprintf(`
		SELECT wh.api_url, wet.event_name, wi.dataset_id
		FROM "%s".webhooks AS wh
		INNER JOIN "%s".webhook_event_subscriptions as wes ON wh.id=wes.webhook_id
		INNER JOIN "%s".dataset_integrations as wi ON wh.id=wi.webhook_id
		INNER JOIN "%s".webhook_event_types as wet ON wes.webhook_event_type_id=wet.id
	`, orgID, orgID, orgID, orgID)

	results, err := query(db, command)
	if err != nil {
		log.Println("Query error:", err)
		return
	}

	webhookCache[orgID] = WebhookCache{
		LastUpdated: time.Now(),
		Webhooks:    results,
	}

	log.Println("REFRESHING CACHE:", webhookCache)
}

type EventMessage struct {
	OrganizationID string                 `json:"organizationId"`
	DatasetID      int                    `json:"datasetId"`
	EventCategory  string                 `json:"eventCategory"`
	EventType      string                 `json:"eventType"`
	Raw            map[string]interface{} `json:"-"`
}

func mapEvents(events map[string]interface{}) (map[string][]EventMessage, bool) {
	mapped := make(map[string][]EventMessage)
	forceRefresh := false

	records := events["Records"].([]interface{})

	for _, r := range records {
		rec := r.(map[string]interface{})
		body := rec["body"].(string)

		var bodyJSON map[string]string
		json.Unmarshal([]byte(body), &bodyJSON)

		var msg EventMessage
		json.Unmarshal([]byte(bodyJSON["Message"]), &msg)

		mapped[msg.OrganizationID] = append(mapped[msg.OrganizationID], msg)

		if msg.EventType == "CREATE_DATASET" {
			forceRefresh = true
		}
	}

	return mapped, forceRefresh
}

type WebhookMessage struct {
	Messages []EventMessage
	URLs     []string
}

func mapWebhookMessages(mapped map[string][]EventMessage, forceRefresh bool) map[string]WebhookMessage {
	result := make(map[string]WebhookMessage)

	for orgID, events := range mapped {

		cacheMutex.Lock()
		cacheEntry, exists := webhookCache[orgID]
		cacheMutex.Unlock()

		if forceRefresh || !exists || time.Since(cacheEntry.LastUpdated) > 10*time.Minute {
			refreshWebhookCache(orgID)
		}

		cacheMutex.Lock()
		webhooks := webhookCache[orgID].Webhooks
		cacheMutex.Unlock()

		for _, evt := range events {
			key := fmt.Sprintf("%d:%s", evt.DatasetID, evt.EventCategory)

			entry := result[key]
			entry.Messages = append(entry.Messages, evt)

			for _, w := range webhooks {
				if w.DatasetID == evt.DatasetID && w.EventName == evt.EventCategory {
					entry.URLs = append(entry.URLs, w.APIURL)
				}
			}

			result[key] = entry
		}
	}

	return result
}

func webhookBodyParser(url string, msg EventMessage) []byte {
	if len(url) > 30 && url[:30] == "https://hooks.slack.com/" {
		wrapped := map[string]string{
			"text": string(mustJSON(msg)),
		}
		return mustJSON(wrapped)
	}
	return mustJSON(msg)
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func broadcastMessages(messages map[string]WebhookMessage) {
	for _, record := range messages {
		for _, url := range record.URLs {
			for _, msg := range record.Messages {

				body := webhookBodyParser(url, msg)

				req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				resp, err := httpClient.Do(req)
				if err != nil {
					log.Println("HTTP error:", err)
					continue
				}
				resp.Body.Close()
			}
		}
	}
}

func handler(ctx context.Context, events map[string]interface{}) (map[string]interface{}, error) {
	log.Println("Lambda handler invoked at", time.Now())

	mappedEvents, forceRefresh := mapEvents(events)
	webhookMessages := mapWebhookMessages(mappedEvents, forceRefresh)
	broadcastMessages(webhookMessages)

	return events, nil
}

func main() {
	lambda.Start(handler)
}
