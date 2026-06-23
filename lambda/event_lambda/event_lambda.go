package main

import (

	_ "github.com/lib/pq"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"net/http"

	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

var (
	env           = os.Getenv("ENV")
	webhookCache  = make(map[string]WebhookCache)
	cacheMutex    sync.Mutex
	httpClient    = &http.Client{Timeout: 250 * time.Millisecond}
	ssmClient     *ssm.Client
)

type WebhookCache struct {
	Updated     time.Time
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

	output, paramerr := ssmClient.GetParameter(context.TODO(), input)
	if paramerr != nil {
		log.Fatalf("Error fetching parameter %s: %v", name, paramerr)
	}
	return *output.Parameter.Value
}

func connectDB() (*sql.DB, error) {
	dbname := getSSMParam(fmt.Sprintf("/%s/integration-service/integrations-postgres-db", env), false)
	dbusername := getSSMParam(fmt.Sprintf("/%s/integration-service/integrations-postgres-user", env), false)
	dbpassword := getSSMParam(fmt.Sprintf("/%s/integration-service/integrations-postgres-password", env), true)
	dbhostname := getSSMParam(fmt.Sprintf("/%s/integration-service/integrations-postgres-host", env), false)

	connectionStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		dbhostname, dbusername, dbpassword, dbname)

	db, sqlerr := sql.Open("postgres", connectionStr)
	if sqlerr != nil {
		return nil, sqlerr
	}

	return db, nil
}

func query(db *sql.DB, command string) ([]WebhookRecord, error) {
	rows, dberr := db.Query(command)
	if dberr != nil {
		return nil, dberr
	}
	defer rows.Close()

	var res []WebhookRecord

	for rows.Next() {
		var r WebhookRecord
		err := rows.Scan(&r.APIURL, &r.EventName, &r.DatasetID)
		if err != nil {
			return nil, err
		}
		res = append(res, r)
	}

	return res, nil
}

func refreshWebhookCache(orgID string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	db, sqlerr := connectDB()
	if sqlerr != nil {
		log.Println("Connection error:", sqlerr)
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

	results, dberr := query(db, command)
	if dberr != nil {
		log.Println("Query error:", dberr)
		return
	}

	webhookCache[orgID] = WebhookCache{
		Updated: time.Now(),
		Webhooks:    results,
	}

	log.Println("REFRESHING CACHE:", webhookCache)
}

type EventMessage struct {
	OrgID          string                 `json:"organizationId"`
	DataID         int                    `json:"datasetId"`
	Category       string                 `json:"eventCategory"`
	Type           string                 `json:"eventType"`
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

		mapped[msg.OrgID] = append(mapped[msg.OrgID], msg)

		if msg.Type == "CREATE_DATASET" {
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

		if forceRefresh || !exists || time.Since(cacheEntry.Updated) > 10*time.Minute {
			refreshWebhookCache(orgID)
		}

		cacheMutex.Lock()
		webhooks := webhookCache[orgID].Webhooks
		cacheMutex.Unlock()

		for _, evt := range events {
			key := fmt.Sprintf("%d:%s", evt.DataID, evt.Category)

			entry := result[key]
			entry.Messages = append(entry.Messages, evt)

			for _, w := range webhooks {
				if w.DatasetID == evt.DataID && w.EventName == evt.Category {
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
		wrap := map[string]string{
			"text": string(mustJSON(msg)),
		}
		return mustJSON(wrap)
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

				request, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
				request.Header.Set("Content-Type", "application/json")

				response, httpErr := httpClient.Do(request)
				if httpErr != nil {
					log.Println("HTTP error:", httpErr)
					continue
				}
				response.Body.Close()
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
