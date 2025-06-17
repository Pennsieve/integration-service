package store_dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
)

type NodeDBStore interface {
	GetById(context.Context, string) (Node, error)
}

type NodeDatabaseStore struct {
	DB        *dynamodb.Client
	TableName string
}

func NewNodeDatabaseStore(db *dynamodb.Client, tableName string) NodeDBStore {
	return &NodeDatabaseStore{db, tableName}
}
func (r *NodeDatabaseStore) GetById(ctx context.Context, uuid string) (Node, error) {
	node := Node{Uuid: uuid}
	response, err := r.DB.GetItem(ctx, &dynamodb.GetItemInput{
		Key: node.GetKey(), TableName: aws.String(r.TableName),
	})
	if err != nil {
		return Node{}, fmt.Errorf("error getting node: %w", err)
	}
	if response.Item == nil {
		return Node{}, nil
	}

	err = attributevalue.UnmarshalMap(response.Item, &node)
	if err != nil {
		return node, fmt.Errorf("error unmarshaling node: %w", err)
	}

	return node, nil
}
