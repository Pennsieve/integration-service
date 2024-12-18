package log_retriever

import (
	"encoding/json"

	"github.com/pennsieve/integration-service/service/utils"
)

type ProcessorLogPayload struct {
	Messages []ProcessorLogMessage `json:"messages"`
}

type ProcessorLogMessage struct {
	Timestamp       int64  `json:"timestamp"`
	UtcLocationTime string `json:"utcLocationTime"`
	UtcTime         string `json:"utcTime"`
	Message         string `json:"message"`
	IngestionTime   int64  `json:"ingestionTime"`
}

func (l *ProcessorLogMessage) UnmarshalJSON(b []byte) error {
	var originalResponse struct {
		Timestamp     int64  `json:"timestamp"`
		Message       string `json:"message"`
		IngestionTime int64  `json:"ingestionTime"`
	}
	if err := json.Unmarshal(b, &originalResponse); err != nil {
		return err
	}
	*l = ProcessorLogMessage{
		Timestamp:       originalResponse.Timestamp,
		UtcLocationTime: utils.ConvertEpochToUTCLocation(originalResponse.Timestamp),
		UtcTime:         utils.ConvertEpochToUTCRFC3339(originalResponse.Timestamp),
		Message:         originalResponse.Message,
		IngestionTime:   originalResponse.IngestionTime,
	}
	return nil
}
