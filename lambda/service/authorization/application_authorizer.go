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
	"github.com/pennsieve/pennsieve-go-core/pkg/models/user"
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

	first := isAppEnabledInOrgWithSufficientPermission(ctx, store, integration.ApplicationID, a.claims.OrgClaim)
	// datasetId is optional
	if integration.DatasetID != 0 {
		second := isAppEnabledInDatasetWithSufficientPermission(ctx, store, integration.DatasetID, a.claims.UserClaim)
		return first && second
	}

	return first
}

// is userRole invoking application >= orgRole of application
func isAppEnabledInOrgWithSufficientPermission(ctx context.Context, store store.DatabaseStore, applicationId int64, orgClaim organization.Claim) bool {
	applicationOrganizationUser, err := store.GetOrganizationUserById(ctx, applicationId)
	if err != nil {
		log.Print(err)
		return false
	}

	if applicationOrganizationUser != nil {
		currentUserOrgRole := orgClaim.Role
		if currentUserOrgRole >= applicationOrganizationUser.PermissionBit {
			return true
		}
		log.Print("userOrgRoleLessThanAppUserOrgRole")
	}

	return false
}

// is userDatasetRole >= datasetRole of application
func isAppEnabledInDatasetWithSufficientPermission(ctx context.Context, store store.DatabaseStore, datasetId int64, userClaim user.Claim) bool {
	userID := userClaim.Id
	currentDatasetUser, err := store.GetDatasetUserByUserId(ctx, userID, datasetId)
	if err != nil {
		log.Print(err)
		return false
	}

	applicationDatasetUser, err := store.GetDatasetUserById(ctx, userID, datasetId)
	if err != nil {
		log.Print(err)
		return false
	}

	if currentDatasetUser != nil {
		currentUserOrgRole := currentDatasetUser.Role
		if currentUserOrgRole >= applicationDatasetUser.Role {
			return true
		}
		log.Print("userDatasetRoleLessThanAppUserDatasetRole")
	}
	return false
}
