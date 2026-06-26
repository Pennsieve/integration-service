package utils

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/Pennsieve/integration-service/internal/models"
)

const (
	slackHooksURLPrefix = "https://hooks.slack.com/"
)

func WebhookBodyParser(url string, msg models.EventMessage) ([]byte, error) {
	if strings.HasPrefix(url, slackHooksURLPrefix) {
		return mustJSON(map[string]string{"text": string(mustJSON(msg))}), nil
	}
	return mustJSON(msg), nil
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("JSON error: %v\n", err)
		return []byte("{}")
	}
	return b
}
