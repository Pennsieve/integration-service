package models

type Integration struct {
	SessionToken   string         `json:"sessionToken"`
	DatasetID      string         `json:"datasetId"`
	ApplicationID  int64          `json:"applicationId"`
	TriggerPayload TriggerPayload `json:"payload"`
	TriggerClient  string         `json:"client"`
}

type TriggerPayload struct {
	PackageIDs []int64 `json:"packageIds"`
}
