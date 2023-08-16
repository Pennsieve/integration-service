package authorization_test

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pennsieve/integration-service/service/authorization"
	"github.com/pennsieve/integration-service/service/store"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

func TestIsAuthorized(t *testing.T) {
	// should return false
	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: make(map[string]interface{}),
		},
	}
	request := events.APIGatewayV2HTTPRequest{
		RouteKey:       "POST /integrations",
		Body:           "{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetId\": \"dataset123\", \"applicationId\": 1, \"organizationId\": 1, \"payload\": {\"packageIds\": [1,2,3]}}",
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(request)
	if authorizer.IsAuthorized(context.Background()) {
		t.Fatalf("expected authorizer to return false")
	}

}

func TestIsAppEnabledInOrg(t *testing.T) {
	ctx := context.Background()
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		log.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: make(map[string]interface{}),
		},
	}

	failureRequest := events.APIGatewayV2HTTPRequest{
		RouteKey:       "POST /integrations",
		Body:           "{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetId\": \"dataset123\", \"applicationId\": 0, \"organizationId\": 1, \"payload\": {\"packageIds\": [1,2,3]}}",
		RequestContext: requestContext,
	}

	// should return false if application is NOT enabled in org (organizationUser is not returned)
	authorizer := authorization.NewApplicationAuthorizer(failureRequest)
	if authorizer.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return false")
	}

	// should return true if application is enabled in org
	var organizationId int64 = 1
	applicationDatabaseStore := store.NewApplicationDatabaseStore(db, organizationId)
	mockApplication := store.Application{
		URL:               "http://mock-application:8081/mock",
		Description:       "This is the Mock Application",
		Secret:            "1d611551faddd83b",
		Name:              "CUSTOM_INTEGRATION",
		DisplayName:       "Custom Integration",
		IsPrivate:         true,
		IsDefault:         false,
		IsDisabled:        false,
		CreatedAt:         time.Now(),
		CreatedBy:         1,
		IntegrationUserID: 1,
		HasAccess:         true,
	}
	mockApplicationID, err := applicationDatabaseStore.Insert(ctx, mockApplication)
	if err != nil {
		log.Fatalf("error inserting application %v", err)
	}

	organizationUser := store.OrganizationUser{
		OrganizationID: organizationId,
		UserID:         mockApplication.IntegrationUserID,
		PermissionBit:  8,
	}
	_, err = applicationDatabaseStore.InsertOrganizationUser(ctx, organizationUser)
	if err != nil {
		log.Fatalf("error inserting application %v", err)
	}

	successRequest := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetId\": \"dataset123\", \"applicationId\": %v, \"organizationId\": %v, \"payload\": {\"packageIds\": [1,2,3]}}",
			mockApplicationID, organizationId),
		RequestContext: requestContext,
	}

	authorizer2 := authorization.NewApplicationAuthorizer(successRequest)
	if !authorizer2.IsAuthorized(ctx) {
		// TODO refactor
		// cleanup
		err = applicationDatabaseStore.Delete(ctx, mockApplicationID)
		if err != nil {
			log.Fatal(err)
		}
		err = applicationDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
		if err != nil {
			log.Fatal(err)
		}
		t.Fatalf("expected authorizer to return true")
	}

	// cleanup
	err = applicationDatabaseStore.Delete(ctx, mockApplicationID)
	if err != nil {
		log.Fatal(err)
	}
	err = applicationDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
	if err != nil {
		log.Fatal(err)
	}

}
