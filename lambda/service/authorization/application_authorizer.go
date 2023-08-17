package authorization

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
	"github.com/pennsieve/pennsieve-go-core/pkg/models/organization"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

type ServiceAuthorizer interface {
	IsAuthorized(context.Context) bool
}

type ApplicationAuthorizer struct {
	claims      *authorizer.Claims // datasetClaim from authorizer would be nil, as no datasetId passed as queryParam
	requestBody string
}

func NewApplicationAuthorizer(request events.APIGatewayV2HTTPRequest) ServiceAuthorizer {
	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)

	return &ApplicationAuthorizer{claims, request.Body}
}

func (a *ApplicationAuthorizer) IsAuthorized(ctx context.Context) bool {
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		log.Print(err)
		return false
	}
	defer db.Close()

	var integration models.Integration
	if err := json.Unmarshal([]byte(a.requestBody), &integration); err != nil {
		log.Println(err)
		return false
	}
	store := store.NewApplicationDatabaseStore(db, integration.OrganizationID)

	// datasetId is optional
	return isAppEnabledInOrgWithSufficientPermission(ctx, store, integration.ApplicationID, a.claims.OrgClaim)
}

// is userRole invoking application >= orgRole of application
func isAppEnabledInOrgWithSufficientPermission(ctx context.Context, store store.DatabaseStore, applicationId int64, orgClaim organization.Claim) bool {
	organizationUser, err := store.GetOrganizationUserById(ctx, applicationId)
	if err != nil {
		log.Print(err)
		return false
	}

	if organizationUser != nil {
		currentUserOrgRole := orgClaim.Role
		if currentUserOrgRole >= organizationUser.PermissionBit {
			return true
		}
		log.Print("userOrgRoleLessThanAppUserOrgRole")
	}

	return false
}

func isAppEnabledInDataset() bool {
	return false
}

// is userDatasetRole >= datasetRole of application
func isUserDatasetRoleGreaterThanAppUserDatasetRole() bool {
	// we have to get the datasetRole, similarly to how the authorizer gets it
	return false
}
