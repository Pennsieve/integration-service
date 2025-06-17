package store_dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Node struct {
	Uuid                  string `dynamodbav:"uuid"`
	Name                  string `dynamodbav:"name"`
	Description           string `dynamodbav:"description"`
	ComputeNodeGatewayUrl string `dynamodbav:"computeNodeGatewayUrl"`
	EfsId                 string `dynamodbav:"efsId"`
	QueueUrl              string `dynamodbav:"queueUrl"`
	Env                   string `dynamodbav:"environment"`
	AccountUuid           string `dynamodbav:"accountUuid"`
	AccountId             string `dynamodbav:"accountId"`
	AccountType           string `dynamodbav:"accountType"`
	CreatedAt             string `dynamodbav:"createdAt"`
	OrganizationId        string `dynamodbav:"organizationId"`
	UserId                string `dynamodbav:"userId"`
	Identifier            string `dynamodbav:"identifier"`
	WorkflowManagerTag    string `dynamodbav:"workflowManagerTag"`
}

func (i Node) GetKey() map[string]types.AttributeValue {
	uuid, err := attributevalue.Marshal(i.Uuid)
	if err != nil {
		panic(err)
	}

	return map[string]types.AttributeValue{"uuid": uuid}
}
