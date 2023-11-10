package trigger

type ApplicationPayload struct {
	IntegrationId string      `json:"integrationId"`
	Params        interface{} `json:"params"`
}
