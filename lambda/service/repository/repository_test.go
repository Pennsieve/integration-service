package repository_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/pennsieve/integration-service/service/repository"
	pgQueries "github.com/pennsieve/pennsieve-go-core/pkg/queries/pgdb"
)

func TestGetById(t *testing.T) {
	db, err := pgQueries.ConnectENV()
	if err != nil {
		log.Fatalf("unable to connect to database: %v\n", err)
	}
	defer db.Close()

	var organizationId int64 = 1
	applicationRepository := repository.NewApplicationRepository(db, organizationId)

	mockApplication := repository.Application{
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
	applicationID, err := applicationRepository.Insert(mockApplication)
	if err != nil {
		log.Fatalf("error inserting application %v", err)
	}
	application, err := applicationRepository.GetById(context.Background(), applicationID)
	if err != nil {
		log.Fatalf("error getting application %v", err)
	}
	if application.ID != applicationID {
		log.Fatalf("expected %v, got %v", applicationID, application.ID)
	}
}
