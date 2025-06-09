package store_dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Node struct {
	Uuid                  string `dynamodbav:"uuid"`
	ComputeNodeGatewayUrl string `dynamodbav:"computeNodeGatewayUrl"`
	AccountId             string `dynamodbav:"accountId"`
	AccountType           string `dynamodbav:"accountType"`
}

func (i Node) GetKey() map[string]types.AttributeValue {
	uuid, err := attributevalue.Marshal(i.Uuid)
	if err != nil {
		panic(err)
	}

	return map[string]types.AttributeValue{"uuid": uuid}
}
