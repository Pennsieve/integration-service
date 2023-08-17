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
	// should return false when no records exist in database
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
		Body:           "{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetId\": \"dataset123\", \"applicationId\": 0, \"organizationId\": 0, \"payload\": {\"packageIds\": [1,2,3]}}",
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(request)
	if authorizer.IsAuthorized(context.Background()) {
		t.Fatalf("expected authorizer to return false")
	}

}

func TestIsAppEnabledInOrgWithSufficientPermission(t *testing.T) {
	ctx := context.Background()
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		log.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

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

	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: make(map[string]interface{}),
		},
	}

	failureRequest := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetId\": \"dataset123\", \"applicationId\": %v, \"organizationId\": %v, \"payload\": {\"packageIds\": [1,2,3]}}",
			mockApplicationID, organizationId),
		RequestContext: requestContext,
	}

	// should return false if application exists but is NOT enabled in org (organizationUser is not returned)
	authorizer := authorization.NewApplicationAuthorizer(failureRequest)
	if authorizer.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return false")
	}

	// should return false if application is enabled in org but the invoking user has insufficient rights
	organizationUser := store.OrganizationUser{
		OrganizationID: organizationId,
		UserID:         mockApplication.IntegrationUserID,
		PermissionBit:  8,
	}
	_, err = applicationDatabaseStore.InsertOrganizationUser(ctx, organizationUser)
	if err != nil {
		log.Fatalf("error inserting application %v", err)
	}

	failureRequest2 := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetId\": \"dataset123\", \"applicationId\": %v, \"organizationId\": %v, \"payload\": {\"packageIds\": [1,2,3]}}",
			mockApplicationID, organizationId),
		RequestContext: requestContext,
	}

	authorizer2 := authorization.NewApplicationAuthorizer(failureRequest2)
	if authorizer2.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return false")
	}

	// should return true if application is enabled in org and the invoking user has sufficient rights
	claims := map[string]interface{}{
		"org_claim": map[string]interface{}{
			"Role":            float64(8),
			"IntId":           float64(1),
			"NodeId":          "xyz",
			"EnabledFeatures": nil,
		},
	}

	requestContext2 := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: claims,
		},
	}

	successRequest := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{ \"sessionToken\": \"ae5t678999-a345fgg\", \"datasetId\": \"dataset123\", \"applicationId\": %v, \"organizationId\": %v, \"payload\": {\"packageIds\": [1,2,3]}}",
			mockApplicationID, organizationId),
		RequestContext: requestContext2,
	}

	authorizer3 := authorization.NewApplicationAuthorizer(successRequest)
	if !authorizer3.IsAuthorized(ctx) {
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
