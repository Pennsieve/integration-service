package store_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/pennsieve/integration-service/service/store"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

func TestGetById(t *testing.T) {
	db, err := pgQueries.ConnectENV()
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
	ctx := context.Background()
	applicationID, err := applicationDatabaseStore.Insert(ctx, mockApplication)
	if err != nil {
		log.Fatalf("error inserting application %v", err)
	}
	application, err := applicationDatabaseStore.GetById(ctx, applicationID)
	if err != nil {
		log.Fatalf("error getting application %v", err)
	}
	if application.ID != applicationID {
		log.Fatalf("expected %v, got %v", applicationID, application.ID)
	}

	// delete inserted test application record
	err = applicationDatabaseStore.Delete(ctx, applicationID)
	if err != nil {
		log.Fatal(err)
	}
}
