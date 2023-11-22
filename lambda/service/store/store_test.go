package store_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pennsieve/integration-service/service/store"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

func TestGetById(t *testing.T) {
	db, err := pgQueries.ConnectENV()
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
	ctx := context.Background()
	applicationID, err := applicationDatabaseStore.Insert(ctx, mockApplication)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}
	application, err := applicationDatabaseStore.GetById(ctx, applicationID)
	if err != nil {
		t.Fatalf("error getting application %v", err)
	}
	if application.ID != applicationID {
		t.Fatalf("expected %v, got %v", applicationID, application.ID)
	}

	// delete inserted test application record
	err = applicationDatabaseStore.Delete(ctx, applicationID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetOrganizationUserById(t *testing.T) {
	db, err := pgQueries.ConnectENV()
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
		IntegrationUserID: 3,
		HasAccess:         true,
	}
	ctx := context.Background()
	applicationID, err := applicationDatabaseStore.Insert(ctx, mockApplication)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	organizationUser := store.OrganizationUser{
		OrganizationID: organizationId,
		UserID:         mockApplication.IntegrationUserID,
		PermissionBit:  8,
	}
	_ = testDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
	_, err = testDatabaseStore.InsertOrganizationUser(ctx, organizationUser)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	insertedOrgUser, err := applicationDatabaseStore.GetOrganizationUserById(ctx, applicationID)
	if err != nil {
		t.Fatalf("error getting application %v", err)
	}

	if insertedOrgUser == nil {
		t.Fatalf("expected orgUser to be retrieved")
	}

	// delete inserted test application record
	err = applicationDatabaseStore.Delete(ctx, applicationID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteOrganizationUser(ctx, organizationId, mockApplication.IntegrationUserID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetDatasetUserById(t *testing.T) {
	ctx := context.Background()
	db, err := pgQueries.ConnectENV()
	if err != nil {
		t.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	var organizationId int64 = 1
	applicationDatabaseStore := store.NewApplicationDatabaseStore(db, organizationId)
	testDatabaseStore := store.NewApplicationTestDatabaseStore(db, organizationId)

	user := createNewUser()
	userId, err := testDatabaseStore.InsertUser(ctx, user)
	if err != nil {
		t.Fatalf("error inserting user %v", err)
	}

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
		IntegrationUserID: userId,
		HasAccess:         true,
	}

	applicationID, err := applicationDatabaseStore.Insert(ctx, mockApplication)
	if err != nil {
		t.Fatalf("error inserting application %v", err)
	}

	datasetId := int64(1)
	userDatasetUser := store.DatasetUser{
		DatasetID: datasetId,
		UserID:    userId,
		Role:      "viewer",
	}
	_, err = testDatabaseStore.InsertDatasetUser(ctx, userDatasetUser)
	if err != nil {
		t.Fatalf("error inserting datasetUser %v", err)
	}

	insertedDatasetUser, err := applicationDatabaseStore.GetDatasetUserById(ctx, applicationID, datasetId)
	if err != nil {
		t.Fatalf("error getting application %v", err)
	}

	if insertedDatasetUser == nil {
		t.Fatalf("expected datasetUser to be retrieved")
	}

	// delete inserted test application record
	err = applicationDatabaseStore.Delete(ctx, applicationID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteDatasetUser(ctx, userDatasetUser.DatasetID, userDatasetUser.UserID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteUser(ctx, userId)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetDatasetUserByUserId(t *testing.T) {
	ctx := context.Background()
	db, err := pgQueries.ConnectENV()
	if err != nil {
		t.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	var organizationId int64 = 1
	applicationDatabaseStore := store.NewApplicationDatabaseStore(db, organizationId)
	testDatabaseStore := store.NewApplicationTestDatabaseStore(db, organizationId)

	user := createNewUser()
	userId, err := testDatabaseStore.InsertUser(ctx, user)
	if err != nil {
		t.Fatalf("error inserting user %v", err)
	}

	datasetId := int64(1)
	userDatasetUser := store.DatasetUser{
		DatasetID: datasetId,
		UserID:    userId,
		Role:      "viewer",
	}

	_ = testDatabaseStore.DeleteDatasetUser(ctx, userDatasetUser.DatasetID, userDatasetUser.UserID) // ensure no duplicates
	_, err = testDatabaseStore.InsertDatasetUser(ctx, userDatasetUser)
	if err != nil {
		t.Fatalf("error inserting datasetUser %v", err)
	}

	insertedDatasetUser, err := applicationDatabaseStore.GetDatasetUserByUserId(ctx, userDatasetUser.UserID, userDatasetUser.DatasetID)
	if err != nil {
		t.Fatalf("error getting application %v", err)
	}

	if insertedDatasetUser == nil {
		t.Fatalf("expected datasetUser to be retrieved")
	}

	// cleanup
	err = testDatabaseStore.DeleteDatasetUser(ctx, userDatasetUser.DatasetID, userDatasetUser.UserID)
	if err != nil {
		t.Fatal(err)
	}
	err = testDatabaseStore.DeleteUser(ctx, userId)
	if err != nil {
		t.Fatal(err)
	}

}

func createNewUser() store.User {
	id := uuid.New()
	uuid := id.String()
	randomId := rand.Intn(100)

	return store.User{
		ID:             int64(randomId),
		Email:          fmt.Sprintf("%s@email.com", uuid),
		FirstName:      uuid,
		LastName:       fmt.Sprintf("Test%s", uuid),
		Color:          "#5FBFF9",
		IsSuperAdmin:   false,
		AuthyID:        int64(randomId),
		PreferredOrgID: int64(randomId),
		Status:         true,
		NodeID:         fmt.Sprintf("N:user:%s", uuid),
	}
}
