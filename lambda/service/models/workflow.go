package models

type WorkflowInstance struct {
	Uuid          string      `json:"uuid"`
	Name          string      `json:"name"`
	ApplicationID int64       `json:"applicationId,omitempty"`
	ComputeNode   ComputeNode `json:"computeNode,omitempty"`
	DatasetNodeID string      `json:"datasetId"`
	PackageIDs    []string    `json:"packageIds"`
	Workflow      interface{} `json:"workflow,omitempty"`
	Params        interface{} `json:"params,omitempty"`
	StartedAt     string      `json:"startedAt"`
	CompletedAt   string      `json:"completedAt"`
}

type StatusMetadata struct {
	Uuid        string `json:"uuid"`
	Status      string `json:"status"`
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
}

type WorkflowInstanceStatus struct {
	StatusMetadata
	Processors []WorkflowProcessorStatus `json:"processors"`
}

type WorkflowProcessorStatus struct {
	StatusMetadata
}

type WorkflowInstanceStatusEvent struct {
	Uuid      string `json:"uuid"`
	Status    string `json:"status"`
	Timestamp int    `json:"timestamp"`
}

type ComputeNode struct {
	ComputeNodeUuid       string `json:"uuid"`
	ComputeNodeGatewayUrl string `json:"computeNodeGatewayUrl,omitempty"`
}

type IntegrationResponse struct {
	Message string `json:"message"`
}

type Workflow struct {
	Uuid        string   `json:"uuid"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Processors  []string `json:"processors"`
	CreatedAt   string   `json:"createdAt"`
	CreatedBy   string   `json:"createdBy"`
}
