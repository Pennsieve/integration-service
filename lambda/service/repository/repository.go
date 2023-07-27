package repository

import (
	"context"
	"database/sql"
	"time"
)

type DatabaseRepository interface {
	GetById(context.Context, int64) (Application, error)
	Insert() (int64, error)
}

type ApplicationRepository struct {
	DB             *sql.DB
	OrganizationID int64
}

func NewDatabaseRepository(db *sql.DB, organizationId int64) DatabaseRepository {
	return &ApplicationRepository{db, organizationId}
}

func (r *ApplicationRepository) GetById(ctx context.Context, applicationId int64) (Application, error) {
	query := "SELECT id, name, api_url, is_disabled FROM \"1\".webhooks WHERE id=$1"
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var application Application
	err := r.DB.QueryRowContext(queryContext, query, applicationId).Scan(
		&application.ID,
		&application.Name,
		&application.URL,
		&application.IsDisabled)
	if err != nil {
		return Application{}, err
	}

	if application.ID != applicationId {
		return Application{}, err
	}
	return application, nil

}

// TODO: update this method to be generic
func (r *ApplicationRepository) Insert() (int64, error) {
	var id int64
	if err := r.DB.QueryRow(
		"insert into \"1\".webhooks (api_url,description,secret,name,display_name,is_private,is_default,is_disabled,created_at,created_by,integration_user_id,has_access) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING ID",
		"http://mock-application:8081/mock", "This is the Mock Application", "1d611551faddd83b", "CUSTOM_INTEGRATION", "Custom Integration", true, false, false, "2023-05-31 17:11:14.634542", 1, 1, true).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}
