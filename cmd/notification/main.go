package main

import (
	"github.com/Pennsieve/integration-service/internal/handler"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler.NotificationHandler)
}
