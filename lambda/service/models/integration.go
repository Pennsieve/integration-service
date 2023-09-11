package models

type Integration struct {
	ApplicationID int64       `json:"applicationId"`
	DatasetNodeID string      `json:"datasetId"`
	PackageIDs    []string    `json:"packageIds"`
	Params        interface{} `json:"params"`
}
