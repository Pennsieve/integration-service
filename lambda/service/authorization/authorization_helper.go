package authorization

import (
	"database/sql"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/pennsieve-go-core/pkg/authorizer"
)

type AuthorizationHelper interface {
	IsAuthorized() bool
	// to be removed
	IsAppEnabledInOrg() bool
	IsAppEnabledInDataset() bool
	IsInvokingUserOrgRoleGreaterThanAppUserOrgRole() bool
	IsInvokingUserDatasetRoleGreaterThanAppUserDatasetRole() bool
}

type ClaimsAuthorizationHelper struct {
	claims *authorizer.Claims // datasetClaim from authorizer would be nil, as no datasetId passed as queryParam
	db     *sql.DB
}

func NewClaimsAuthorizationHelper(request events.APIGatewayV2HTTPRequest, db *sql.DB) AuthorizationHelper {
	claims := authorizer.ParseClaims(request.RequestContext.Authorizer.Lambda)
	return &ClaimsAuthorizationHelper{claims, db}
}

func (a *ClaimsAuthorizationHelper) IsAuthorized() bool {
	// datasetId is optional
	return a.IsAppEnabledInOrg() && a.IsAppEnabledInDataset() &&
		a.IsInvokingUserOrgRoleGreaterThanAppUserOrgRole() &&
		a.IsInvokingUserDatasetRoleGreaterThanAppUserDatasetRole()
}

func (a *ClaimsAuthorizationHelper) IsAppEnabledInOrg() bool {
	// Re-confirm/re-visit use of isDisabled field in the webhooks table?
	return false
}

func (a *ClaimsAuthorizationHelper) IsAppEnabledInDataset() bool {
	return false
}

// is userRole invoking application >= orgRole of application
func (a *ClaimsAuthorizationHelper) IsInvokingUserOrgRoleGreaterThanAppUserOrgRole() bool {
	return false
}

// is userDatasetRole >= datasetRole of application
func (a *ClaimsAuthorizationHelper) IsInvokingUserDatasetRoleGreaterThanAppUserDatasetRole() bool {
	// we have to get the datasetRole, similarly to how the authorizer gets it
	return false
}
