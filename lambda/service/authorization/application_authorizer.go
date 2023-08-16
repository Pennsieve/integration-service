package authorization

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

type ServiceAuthorizer interface {
	IsAuthorized(context.Context) bool
}

type ApplicationAuthorizer struct {
	claims  *authorizer.Claims // datasetClaim from authorizer would be nil, as no datasetId passed as queryParam
	request events.APIGatewayV2HTTPRequest
}

func NewApplicationAuthorizer(request events.APIGatewayV2HTTPRequest) ServiceAuthorizer {
	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)
	return &ApplicationAuthorizer{claims, request}
}

func (a *ApplicationAuthorizer) IsAuthorized(ctx context.Context) bool {
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		log.Print(err)
		return false
	}
	defer db.Close()

	var integration models.Integration
	if err := json.Unmarshal([]byte(a.request.Body), &integration); err != nil {
		log.Println(err)
		return false
	}
	store := store.NewApplicationDatabaseStore(db, integration.OrganizationID)

	// datasetId is optional
	return a.isAppEnabledInOrg(ctx, store, integration.ApplicationID)
}

func (a *ApplicationAuthorizer) isAppEnabledInOrg(ctx context.Context, store store.DatabaseStore, applicationId int64) bool {
	// Re-confirm/re-visit use of isDisabled field in the webhooks table?
	organizationUser, err := store.GetOrganizationUserById(ctx, applicationId)
	if err != nil {
		log.Print(err)
		return false
	}
	if organizationUser != nil {
		return true
	}

	return false
}

func isAppEnabledInDataset() bool {
	return false
}

// is userRole invoking application >= orgRole of application
func isUserOrgRoleGreaterThanAppUserOrgRole() bool {
	return false
}

// is userDatasetRole >= datasetRole of application
func isUserDatasetRoleGreaterThanAppUserDatasetRole() bool {
	// we have to get the datasetRole, similarly to how the authorizer gets it
	return false
}
