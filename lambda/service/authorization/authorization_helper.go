package authorization

import (
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
	"github.com/pennsieve/pennsieve-go-core/pkg/models/permissions"
)

type AuthorizationHelper interface {
	IsAuthorized() bool
}

type ClaimsAuthorizationHelper struct {
	claims        *authorizer.Claims
	requestMethod string
}

func NewClaimsAuthorizationHelper(request events.APIGatewayV2HTTPRequest) AuthorizationHelper {
	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)
	return &ClaimsAuthorizationHelper{claims, request.RequestContext.HTTP.Method}
}

func (a *ClaimsAuthorizationHelper) IsAuthorized() bool {
	switch a.requestMethod {
	case http.MethodPost, http.MethodDelete:
		return authorizer.HasRole(*a.claims, permissions.CreateDeleteFiles)
	case http.MethodGet:
		return authorizer.HasRole(*a.claims, permissions.ViewFiles)
	default:
		log.Print("unsupported path")
		return false
	}
}