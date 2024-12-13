package log_retriever

type ProcessorLogPayload struct {
	Messages []ProcessorLogMessage `json:"messages"`
}

type ProcessorLogMessage struct {
	Timestamp     int64  `json:"timestamp"`
	Message       string `json:"message"`
	IngestionTime int64  `json:"ingestionTime"`
}
