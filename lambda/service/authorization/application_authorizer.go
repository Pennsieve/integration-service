package authorization

import (
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

type ServiceAuthorizer interface {
	IsAuthorized() bool
}

type ApplicationAuthorizer struct {
	claims *authorizer.Claims // datasetClaim from authorizer would be nil, as no datasetId passed as queryParam
}

func NewApplicationAuthorizer(request events.APIGatewayV2HTTPRequest) ServiceAuthorizer {
	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)
	return &ApplicationAuthorizer{claims}
}

func (a *ApplicationAuthorizer) IsAuthorized() bool {
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		log.Print(err)
		return false
	}
	defer db.Close()

	// datasetId is optional
	return isAppEnabledInOrg() && isAppEnabledInDataset() &&
		isUserOrgRoleGreaterThanAppUserOrgRole() &&
		isUserDatasetRoleGreaterThanAppUserDatasetRole()
}

func isAppEnabledInOrg() bool {
	// Re-confirm/re-visit use of isDisabled field in the webhooks table?
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
