package models

type Integration struct {
	SessionToken   string         `json:"sessionToken"`
	DatasetID      string         `json:"datasetId"`
	ApplicationID  int64          `json:"applicationId"`
	OrganizationID int64          `json:"organizationId"`
	TriggerPayload TriggerPayload `json:"payload"`
}

type TriggerPayload struct {
	PackageIDs []int64 `json:"packageIds"`
}
