package store

import "github.com/pennsieve/integration-service/service/models"

type Store interface {
	GetById(int64) (models.Application, error)
}

type ApplicationStore struct {
}

func NewStore() Store {
	return &ApplicationStore{}
}

func (r *ApplicationStore) GetById(applicationId int64) (models.Application, error) {
	return models.Application{
		ID:         1,
		Name:       "mockApplication",
		URL:        "http://localhost:8081/mock",
		IsActive:   true,
		IsInternal: false,
	}, nil
}
