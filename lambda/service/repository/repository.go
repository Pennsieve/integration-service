package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type DatabaseRepository interface {
	GetById(context.Context, int64) (Application, error)
	Insert(Application) (int64, error)
}

type ApplicationRepository struct {
	DB             *sql.DB
	OrganizationID int64
}

func NewApplicationRepository(db *sql.DB, organizationId int64) DatabaseRepository {
	return &ApplicationRepository{db, organizationId}
}

func (r *ApplicationRepository) GetById(ctx context.Context, applicationId int64) (Application, error) {
	query := fmt.Sprintf("SELECT id, name, api_url, is_disabled FROM \"%v\".webhooks WHERE id=$1",
		r.OrganizationID)
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
func (r *ApplicationRepository) Insert(application Application) (int64, error) {
	var id int64
	query := fmt.Sprintf("insert into \"%v\".webhooks (api_url,description,secret,name,display_name,is_private,is_default,is_disabled,created_at,created_by,integration_user_id,has_access) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING ID",
		r.OrganizationID)
	if err := r.DB.QueryRow(
		query,
		application.URL, application.Description, application.Secret,
		application.Name, application.DisplayName, application.IsPrivate,
		application.IsDefault, application.IsDisabled, application.CreatedAt,
		application.CreatedBy, application.IntegrationUserID, application.HasAccess).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}
