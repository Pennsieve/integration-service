package authorization

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/models"
	"github.com/pennsieve/integration-service/service/store"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
	"github.com/pennsieve/pennsieve-go-core/pkg/models/dataset/role"
	"github.com/pennsieve/pennsieve-go-core/pkg/models/organization"
	"github.com/pennsieve/pennsieve-go-core/pkg/models/user"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

type ServiceAuthorizer interface {
	IsAuthorized(context.Context) bool
}

type ApplicationAuthorizer struct {
	claims      *authorizer.Claims
	requestBody string
	logger      *slog.Logger
}

func NewApplicationAuthorizer(request events.APIGatewayV2HTTPRequest, logger *slog.Logger) ServiceAuthorizer {
	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)

	return &ApplicationAuthorizer{claims, request.Body, logger}
}

func (a *ApplicationAuthorizer) IsAuthorized(ctx context.Context) bool {
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		a.logger.ErrorContext(ctx, err.Error())
		return false
	}
	defer db.Close()

	var integration models.Integration
	if err := json.Unmarshal([]byte(a.requestBody), &integration); err != nil {
		a.logger.ErrorContext(ctx, err.Error())
		return false
	}
	store := store.NewApplicationDatabaseStore(db, integration.OrganizationID)

	isAppEnabledInOrg := isAppEnabledInOrgWithSufficientPermission(ctx, store, integration.ApplicationID, a.claims.OrgClaim, a.logger)
	// datasetId is optional
	if integration.DatasetID != 0 {
		isAppEnabledInDataset := isAppEnabledInDatasetWithSufficientPermission(ctx, store, integration.DatasetID, a.claims.UserClaim, integration.ApplicationID, a.logger)
		return isAppEnabledInOrg && isAppEnabledInDataset
	}

	return isAppEnabledInOrg
}

// is userRole invoking application >= orgRole of application
func isAppEnabledInOrgWithSufficientPermission(ctx context.Context, store store.DatabaseStore, applicationId int64, orgClaim organization.Claim, logger *slog.Logger) bool {
	applicationOrganizationUser, err := store.GetOrganizationUserById(ctx, applicationId)
	if err != nil {
		logger.ErrorContext(ctx, err.Error())
		return false
	}

	// TODO: adding actual roles in organisation_user table
	if applicationOrganizationUser != nil {
		currentUserOrgRole := orgClaim.Role
		if currentUserOrgRole >= applicationOrganizationUser.PermissionBit {
			return true
		}
		logger.Warn("userOrgRoleLessThanAppUserOrgRole")
	}

	return false
}

// is userDatasetRole >= datasetRole of application
func isAppEnabledInDatasetWithSufficientPermission(ctx context.Context, store store.DatabaseStore, datasetId int64, userClaim user.Claim, applicationId int64, logger *slog.Logger) bool {
	// currently datasetClaim from authorizer would be nil, as no datasetId is passed as a queryParam
	currentDatasetUser, err := store.GetDatasetUserByUserId(ctx, userClaim.Id, datasetId)
	if err != nil {
		logger.ErrorContext(ctx, err.Error())
		return false
	}

	applicationDatasetUser, err := store.GetDatasetUserById(ctx, applicationId, datasetId)
	if err != nil {
		logger.ErrorContext(ctx, err.Error())
		return false
	}

	if currentDatasetUser != nil {
		currentUserOrgRoleString := currentDatasetUser.Role
		currentUserOrgRole, ok := role.RoleFromString(currentUserOrgRoleString)
		if !ok {
			logger.Warn("currentUserOrgRole: could not map role from database string")
			return false
		}
		applicationDatasetUserRole, ok := role.RoleFromString(applicationDatasetUser.Role)
		if !ok {
			logger.Warn("applicationDatasetUserRole: could not map role from database string")
			return false
		}
		if currentUserOrgRole >= applicationDatasetUserRole {
			return true
		}
		logger.Warn("userDatasetRoleLessThanAppUserDatasetRole")
	}
	return false
}
