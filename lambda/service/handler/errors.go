package handler

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/models"
)

var ErrApplicationValidation = errors.New("application cannot be triggered")
var ErrRunningTrigger = errors.New("error running trigger")
var ErrUnmarshaling = errors.New("error unmarshalling request body")
var ErrUnsupportedRoute = errors.New("unsupported route")
var ErrDatabaseConnection = errors.New("error connecting to database")
var ErrNoRecordsFound = errors.New("error no records found")
var ErrUnsupportedPath = errors.New("unsupported path")
var ErrUnauthorized = errors.New("not authorized to perform this action")
var ErrConfig = errors.New("error loading AWS config")
var ErrMarshaling = errors.New("error marshalling item")
var ErrDynamoDB = errors.New("error performing action on DynamoDB table")
var ErrMissingParameter = errors.New("missing parameter")
var ErrRunningLogRetriever = errors.New("error running log retriever")

func handlerError(handlerName string, errorMessage error) string {
	log.Printf("%s: %s", handlerName, errorMessage.Error())
	m, err := json.Marshal(models.IntegrationResponse{
		Message: errorMessage.Error(),
	})
	if err != nil {
		log.Printf("%s: %s", handlerName, err.Error())
		return err.Error()
	}

	return string(m)
}

func APIErrorResponse(handler string, code int, message string, err error) events.APIGatewayV2HTTPResponse {
	if err != nil {
		log.Printf("%s: %v", handler, err)
	} else {
		log.Printf("%s: %s", handler, message)
	}

	response := models.IntegrationResponse{
		Message: message,
	}

	return APIJsonResponse(handler, code, response)
}
