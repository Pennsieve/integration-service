package models

type Integration struct {
	Uuid          string      `json:"uuid"`
	ApplicationID int64       `json:"applicationId,omitempty"`
	ComputeNode   ComputeNode `json:"computeNode,omitempty"`
	DatasetNodeID string      `json:"datasetId"`
	PackageIDs    []string    `json:"packageIds"`
	Workflow      interface{} `json:"workflow,omitempty"`
	Params        interface{} `json:"params,omitempty"`
}

type ComputeNode struct {
	ComputeNodeUuid       string `json:"uuid"`
	ComputeNodeGatewayUrl string `json:"computeNodeGatewayUrl,omitempty"`
}
