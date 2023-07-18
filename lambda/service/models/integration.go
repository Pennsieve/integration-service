package model

type Integration struct {
	SessionToken  string            `json:"sessionToken"`
	DatasetID     string            `json:"datasetId"`
	ApplicationID int64             `json:"applicationId"`
	Params        ApplicationParams `json:"params"`
}

type ApplicationParams struct {
	PackageIDs []int64 `json:"packageIds"`
}
