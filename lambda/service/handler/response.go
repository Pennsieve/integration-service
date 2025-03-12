package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

func APIGatewayV2HTTPJsonResponse() events.APIGatewayV2HTTPResponse {
	return events.APIGatewayV2HTTPResponse{
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
}

func APIJsonResponse(handler string, code int, body any) events.APIGatewayV2HTTPResponse {
	response := APIGatewayV2HTTPJsonResponse()
	response.StatusCode = code

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			log.Printf("%s - error while marshalling JSON response: %v", handler, err)

			response.StatusCode = http.StatusInternalServerError
			response.Body = `{"message": "failed to serialize response"}`
			return response
		}
		response.Body = string(jsonBody)
	}

	return response
}
