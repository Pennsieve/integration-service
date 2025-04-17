package models

import "slices"

type WorkflowInstance struct {
	Uuid          string      `json:"uuid"`
	Name          string      `json:"name"`
	ApplicationID int64       `json:"applicationId,omitempty"`
	ComputeNode   ComputeNode `json:"computeNode,omitempty"`
	DatasetNodeID string      `json:"datasetId"`
	PackageIDs    []string    `json:"packageIds"`
	Workflow      interface{} `json:"workflow,omitempty"`
	Params        interface{} `json:"params,omitempty"`
	Status        string      `json:"status"`
	StartedAt     string      `json:"startedAt"`
	CompletedAt   string      `json:"completedAt"`
}

type WorkflowProcessor struct {
	Uuid                     string                 `json:"uuid"`
	ApplicationID            string                 `json:"applicationId"`
	ApplicationContainerName string                 `json:"applicationContainerName"`
	ApplicationType          string                 `json:"applicationType"`
	Params                   map[string]interface{} `json:"params"`
	CommandArguments         []string               `json:"commandArguments"`
}

type StatusMetadata struct {
	Uuid        string `json:"uuid"`
	Status      string `json:"status"`
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
}

type WorkflowInstanceStatus struct {
	StatusMetadata
	Processors []WorkflowProcessorStatus `json:"workflow"`
}

type WorkflowProcessorStatus struct {
	StatusMetadata
}

type WorkflowInstanceStatusEvent struct {
	Status    string `json:"status"`
	Timestamp int    `json:"timestamp"`
}

const (
	WorkflowInstanceStatusNotStarted = "NOT_STARTED"
	WorkflowInstanceStatusStarted    = "STARTED"
	WorkflowInstanceStatusCanceling  = "CANCELING"
	WorkflowInstanceStatusCanceled   = "CANCELED"
	WorkflowInstanceStatusSucceeded  = "SUCCEEDED"
	WorkflowInstanceStatusFailed     = "FAILED"
)

func IsValidWorkflowInstanceStatus(status string) bool {
	return slices.Contains([]string{
		WorkflowInstanceStatusNotStarted,
		WorkflowInstanceStatusStarted,
		WorkflowInstanceStatusCanceling,
		WorkflowInstanceStatusCanceled,
		WorkflowInstanceStatusSucceeded,
		WorkflowInstanceStatusFailed,
	}, status)
}

func IsEndStateWorkflowInstanceStatus(status string) bool {
	return slices.Contains([]string{
		WorkflowInstanceStatusCanceled,
		WorkflowInstanceStatusSucceeded,
		WorkflowInstanceStatusFailed,
	}, status)
}

type ComputeNode struct {
	ComputeNodeUuid       string `json:"uuid"`
	ComputeNodeGatewayUrl string `json:"computeNodeGatewayUrl,omitempty"`
}

type IntegrationResponse struct {
	Message string `json:"message"`
}

type Workflow struct {
	Uuid        string       `json:"uuid"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Processors  []Processors `json:"processors"`
	CreatedAt   string       `json:"createdAt"`
	CreatedBy   string       `json:"createdBy"`
}

type Processors struct {
	Uuid                     string
	Name                     string
	Description              string
	ApplicationContainerName string
	ApplicationId            string // taskDefnId
	// ApplicationType          string // processor, pre, post
	// "params": string
	// "commandArguments": []string,
	InputDataTypeUuid  string
	OutputDataTypeUuid string
	CodeRepoUuid       string
	DependsOn          []string
}
