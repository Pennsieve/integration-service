package authorization_test

import (
	"context"
	"fmt"
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
		Body:           "{ \"datasetId\": \"N:dataset:c0f0db41-c7cb-4fb5-98b4-e90791f8a975\", \"applicationId\": 0, \"packageIds\": [\"1\",\"2\",\"3\"] }",
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(request)
	if authorizer.IsAuthorized(context.Background()) {
		t.Fatalf("expected authorizer to return false")
	}

}

func TestCase1IsAppEnabledInOrgWithSufficientPermission(t *testing.T) {
	// should return false if application exists but is NOT enabled in org (organizationUser is not returned)
	ctx := context.Background()
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		t.Fatalf("unable to connect to database: %v\n", err)
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
		t.Fatalf("error inserting application %v", err)
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
		Body: fmt.Sprintf("{ \"applicationId\": %v, \"packageIds\": [\"1\",\"2\",\"3\"]}",
			mockApplicationID),
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(failureRequest)
	if authorizer.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return false")
	}

	// cleanup
	err = applicationDatabaseStore.Delete(ctx, mockApplicationID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCase2IsAppEnabledInOrgWithSufficientPermission(t *testing.T) {
	// should return false if application is enabled in org but the invoking user has insufficient rights
	ctx := context.Background()
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		t.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	var organizationId int64 = 1
	applicationDatabaseStore := store.NewApplicationDatabaseStore(db, organizationId)
	testDatabaseStore := store.NewApplicationTestDatabaseStore(db, organizationId)
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
		IntegrationUserID: 2,
		HasAccess:         true,
	}
	mockApplicationID, err := applicationDatabaseStore.Insert(ctx, mockApplication)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: make(map[string]interface{}),
		},
	}

	organizationUser := store.OrganizationUser{
		OrganizationID: organizationId,
		UserID:         mockApplication.IntegrationUserID,
		PermissionBit:  8,
	}
	_, err = testDatabaseStore.InsertOrganizationUser(ctx, organizationUser)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	failureRequest := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{ \"applicationId\": %v, \"packageIds\": [\"1\",\"2\",\"3\"]}",
			mockApplicationID),
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(failureRequest)
	if authorizer.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return false")
	}

	// cleanup
	err = applicationDatabaseStore.Delete(ctx, mockApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCase3IsAppEnabledInOrgWithSufficientPermission(t *testing.T) {
	// should return true if application is enabled in org and the invoking user has sufficient rights
	ctx := context.Background()
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		t.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	var organizationId int64 = 1
	applicationDatabaseStore := store.NewApplicationDatabaseStore(db, organizationId)
	testDatabaseStore := store.NewApplicationTestDatabaseStore(db, organizationId)
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
		t.Fatalf("error inserting application %v", err)
	}

	organizationUser := store.OrganizationUser{
		OrganizationID: organizationId,
		UserID:         mockApplication.IntegrationUserID,
		PermissionBit:  8,
	}
	_, err = testDatabaseStore.InsertOrganizationUser(ctx, organizationUser)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	claims := map[string]interface{}{
		"org_claim": map[string]interface{}{
			"Role":            float64(8),
			"IntId":           float64(1),
			"NodeId":          "xyz",
			"EnabledFeatures": nil,
		},
	}

	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: claims,
		},
	}

	successRequest := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{ \"applicationId\": %v, \"packageIds\": [\"1\",\"2\",\"3\"]}",
			mockApplicationID),
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(successRequest)
	if !authorizer.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return true")
	}

	// cleanup
	err = applicationDatabaseStore.Delete(ctx, mockApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCase1IsAppEnabledInDatasetWithSufficientPermission(t *testing.T) {
	// should return true if application is enabled in org and the invoking user has sufficient rights
	// and no datasetId provided
	ctx := context.Background()
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		t.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	var organizationId int64 = 1
	applicationDatabaseStore := store.NewApplicationDatabaseStore(db, organizationId)
	testDatabaseStore := store.NewApplicationTestDatabaseStore(db, organizationId)
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
		t.Fatalf("error inserting application %v", err)
	}

	organizationUser := store.OrganizationUser{
		OrganizationID: organizationId,
		UserID:         mockApplication.IntegrationUserID,
		PermissionBit:  8,
	}
	_, err = testDatabaseStore.InsertOrganizationUser(ctx, organizationUser)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	claims := map[string]interface{}{
		"org_claim": map[string]interface{}{
			"Role":            float64(8),
			"IntId":           float64(1),
			"NodeId":          "xyz",
			"EnabledFeatures": nil,
		},
	}

	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: claims,
		},
	}

	successRequest := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{\"applicationId\": %v, \"packageIds\": [\"1\",\"2\",\"3\"]}",
			mockApplicationID),
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(successRequest)
	if !authorizer.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return true")
	}

	// cleanup
	err = applicationDatabaseStore.Delete(ctx, mockApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
	if err != nil {
		t.Fatal(err)
	}

}

func TestCase2IsAppEnabledInDatasetWithSufficientPermission(t *testing.T) {
	// should return false if application is enabled in org and the invoking user has sufficient rights
	// and a datasetId provided, but app not enabled in dataset
	ctx := context.Background()
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		t.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	var organizationId int64 = 1
	applicationDatabaseStore := store.NewApplicationDatabaseStore(db, organizationId)
	testDatabaseStore := store.NewApplicationTestDatabaseStore(db, organizationId)
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
		t.Fatalf("error inserting application %v", err)
	}

	organizationUser := store.OrganizationUser{
		OrganizationID: organizationId,
		UserID:         mockApplication.IntegrationUserID,
		PermissionBit:  8,
	}
	_, err = testDatabaseStore.InsertOrganizationUser(ctx, organizationUser)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	claims := map[string]interface{}{
		"org_claim": map[string]interface{}{
			"Role":            float64(8),
			"IntId":           float64(1),
			"NodeId":          "xyz",
			"EnabledFeatures": nil,
		},
	}

	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: claims,
		},
	}

	failureRequest := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{ \"applicationId\": %v, \"datasetId\": \"N:dataset:c0f0db41-c7cb-4fb5-98b4-e90791f8a975\", \"packageIds\": [\"1\",\"2\",\"3\"]}",
			mockApplicationID),
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(failureRequest)
	if authorizer.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return false")
	}

	// cleanup
	err = applicationDatabaseStore.Delete(ctx, mockApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
	if err != nil {
		t.Fatal(err)
	}

}

func TestCase3IsAppEnabledInDatasetWithSufficientPermission(t *testing.T) {
	// should return false if application is enabled in org and the invoking user has sufficient rights
	// and a datasetId provided, but app is enabled in dataset, without sufficient permissions
	ctx := context.Background()
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		t.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	var organizationId int64 = 1
	applicationDatabaseStore := store.NewApplicationDatabaseStore(db, organizationId)
	testDatabaseStore := store.NewApplicationTestDatabaseStore(db, organizationId)
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
		t.Fatalf("error inserting application %v", err)
	}

	organizationUser := store.OrganizationUser{
		OrganizationID: organizationId,
		UserID:         mockApplication.IntegrationUserID,
		PermissionBit:  8,
	}
	_, err = testDatabaseStore.InsertOrganizationUser(ctx, organizationUser)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	claims := map[string]interface{}{
		"org_claim": map[string]interface{}{
			"Role":            float64(8),
			"IntId":           float64(1),
			"NodeId":          "xyz",
			"EnabledFeatures": nil,
		},
		"user_claim": map[string]interface{}{
			"Id":           float64(2),
			"NodeId":       "xyz",
			"IsSuperAdmin": false,
		},
	}

	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: claims,
		},
	}

	datasetId := int64(1)
	userDatasetUser := store.DatasetUser{
		DatasetID: datasetId,
		UserID:    2,
		Role:      "viewer",
	}
	_, err = testDatabaseStore.InsertDatasetUser(ctx, userDatasetUser)
	if err != nil {
		t.Fatalf("error inserting datasetUser %v", err)
	}

	failureRequest := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{ \"applicationId\": %v, \"datasetId\": \"N:dataset:c0f0db41-c7cb-4fb5-98b4-e90791f8a975\", \"packageIds\": [\"1\",\"2\",\"3\"]}",
			mockApplicationID),
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(failureRequest)
	if authorizer.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return false")
	}

	// cleanup
	err = applicationDatabaseStore.Delete(ctx, mockApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteDatasetUser(ctx, userDatasetUser.DatasetID, userDatasetUser.UserID)
	if err != nil {
		t.Fatal(err)
	}

}

func TestCase4IsAppEnabledInDatasetWithSufficientPermission(t *testing.T) {
	// should return true if application is enabled in org and the invoking user has sufficient rights
	// and a datasetId provided, app is enabled in dataset, with sufficient permissions
	ctx := context.Background()
	db, err := pgQueries.ConnectRDS()
	if err != nil {
		t.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	var organizationId int64 = 1
	applicationDatabaseStore := store.NewApplicationDatabaseStore(db, organizationId)
	testDatabaseStore := store.NewApplicationTestDatabaseStore(db, organizationId)
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
		t.Fatalf("error inserting application %v", err)
	}

	organizationUser := store.OrganizationUser{
		OrganizationID: organizationId,
		UserID:         mockApplication.IntegrationUserID,
		PermissionBit:  8,
	}
	_, err = testDatabaseStore.InsertOrganizationUser(ctx, organizationUser)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	claims := map[string]interface{}{
		"org_claim": map[string]interface{}{
			"Role":            float64(8),
			"IntId":           float64(1),
			"NodeId":          "xyz",
			"EnabledFeatures": nil,
		},
		"user_claim": map[string]interface{}{
			"Id":           float64(2),
			"NodeId":       "xyz",
			"IsSuperAdmin": false,
		},
	}

	requestContext := events.APIGatewayV2HTTPRequestContext{
		HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
			Method: "POST",
		},
		Authorizer: &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
			Lambda: claims,
		},
	}

	datasetId := int64(1)
	userDatasetUser := store.DatasetUser{
		DatasetID: datasetId,
		UserID:    2,
		Role:      "owner",
	}
	_, err = testDatabaseStore.InsertDatasetUser(ctx, userDatasetUser)
	if err != nil {
		t.Fatalf("error inserting datasetUser %v", err)
	}

	successRequest := events.APIGatewayV2HTTPRequest{
		RouteKey: "POST /integrations",
		Body: fmt.Sprintf("{ \"applicationId\": %v, \"datasetId\": \"N:dataset:c0f0db41-c7cb-4fb5-98b4-e90791f8a975\", \"packageIds\": [\"1\"]}",
			mockApplicationID),
		RequestContext: requestContext,
	}

	authorizer := authorization.NewApplicationAuthorizer(successRequest)
	if !authorizer.IsAuthorized(ctx) {
		t.Fatalf("expected authorizer to return true")
	}

	// cleanup
	err = applicationDatabaseStore.Delete(ctx, mockApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteDatasetUser(ctx, userDatasetUser.DatasetID, userDatasetUser.UserID)
	if err != nil {
		t.Fatal(err)
	}

}
