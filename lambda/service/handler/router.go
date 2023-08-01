package handler

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/utils"
)

type Router interface {
	POST(string, func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error))
	GET(string, func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error))
	Start(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)
}

type LambdaRouter struct {
	GetRoutes  map[string]func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)
	PostRoutes map[string]func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)
}

func NewLambdaRouter() Router {
	return &LambdaRouter{make(map[string]func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)),
		make(map[string]func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error))}
}

func (r *LambdaRouter) POST(routeKey string, handler func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)) {
	r.PostRoutes[routeKey] = handler
}

func (r *LambdaRouter) GET(routeKey string, handler func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)) {
	r.GetRoutes[routeKey] = handler
}

func (r *LambdaRouter) Start(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	routeKey := utils.ExtractRoute(request.RouteKey)
	switch request.RequestContext.HTTP.Method {
	case "POST":
		f, ok := r.PostRoutes[routeKey]
		if ok {
			return f(ctx, request)
		} else {
			return handleError()
		}
	case "GET":
		f, ok := r.GetRoutes[routeKey]
		if ok {
			return f(ctx, request)
		} else {
			return handleError()
		}
	default:
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 404,
			Body:       "LambdaRouter",
		}, ErrUnsupportedRoute
	}
}

func handleError() (events.APIGatewayV2HTTPResponse, error) {
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 404,
		Body:       "LambdaRouter",
	}, ErrUnsupportedRoute
}
