package repository_test

import (
	"context"
	"log"
	"testing"

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
	repository := repository.NewDatabaseRepository(db, organizationId)

	applicationID, err := repository.Insert()
	if err != nil {
		log.Fatalf("error inserting application %v", err)
	}
	application, err := repository.GetById(context.Background(), applicationID)
	if err != nil {
		log.Fatalf("error getting application %v", err)
	}
	if application.ID != applicationID {
		log.Fatalf("expected %v, got %v", applicationID, application.ID)
	}

}
