package store

import "github.com/pennsieve/integration-service/service/models"

type DatabaseStore interface {
	GetById(int64) (models.Application, error)
}

type ApplicationStore struct {
}

func NewStore() DatabaseStore {
	return &ApplicationStore{}
}

func (r *ApplicationStore) GetById(applicationId int64) (models.Application, error) {
	return models.Application{
		ID:         applicationId,
		Name:       "mockApplication",
		URL:        "http://mock-application:8081/mock",
		IsActive:   true,
		IsInternal: false,
	}, nil
}
