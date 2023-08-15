package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type DatabaseStore interface {
	GetById(context.Context, int64) (Application, error)
	Insert(context.Context, Application) (int64, error)
	Delete(context.Context, int64) error
	GetOrganisationUserById(context.Context, int64) (*OrganizationUser, error)
	GetDatasetUserById(context.Context, int64, int64) (*DatasetUser, error)
	GetDatasetUserByUserId(context.Context, int64, int64) (*DatasetUser, error)
}

type ApplicationDatabaseStore struct {
	DB             *sql.DB
	OrganizationID int64
}

func NewApplicationDatabaseStore(db *sql.DB, organizationId int64) DatabaseStore {
	return &ApplicationDatabaseStore{db, organizationId}
}

func (r *ApplicationDatabaseStore) GetById(ctx context.Context, applicationId int64) (Application, error) {
	query := fmt.Sprintf("SELECT id, name, api_url, is_disabled, integration_user_id FROM \"%v\".webhooks WHERE id=$1",
		r.OrganizationID)
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var application Application
	err := r.DB.QueryRowContext(queryContext, query, applicationId).Scan(
		&application.ID,
		&application.Name,
		&application.URL,
		&application.IsDisabled,
		&application.IntegrationUserID)
	if err != nil {
		return Application{}, err
	}

	if application.ID != applicationId {
		return Application{}, err
	}
	return application, nil
}

func (r *ApplicationDatabaseStore) Insert(ctx context.Context, application Application) (int64, error) {
	var id int64
	query := fmt.Sprintf("insert into \"%v\".webhooks (api_url,description,secret,name,display_name,is_private,is_default,is_disabled,created_at,created_by,integration_user_id,has_access) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING ID",
		r.OrganizationID)
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := r.DB.QueryRowContext(
		queryContext,
		query,
		application.URL, application.Description, application.Secret,
		application.Name, application.DisplayName, application.IsPrivate,
		application.IsDefault, application.IsDisabled, application.CreatedAt,
		application.CreatedBy, application.IntegrationUserID, application.HasAccess).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *ApplicationDatabaseStore) Delete(ctx context.Context, applicationId int64) error {
	query := fmt.Sprintf("DELETE from \"%v\".webhooks WHERE id=$1",
		r.OrganizationID)
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := r.DB.QueryRowContext(queryContext, query, applicationId)
	if err != nil {
		return err.Err()
	}
	return nil
}

func (r *ApplicationDatabaseStore) GetOrganisationUserById(ctx context.Context, applicationId int64) (*OrganizationUser, error) {
	query := fmt.Sprintf("SELECT organization_id, user_id, permission_bit from pennsieve.organization_user where organization_id=%[1]v and user_id in (SELECT integration_user_id from \"%[1]v\".webhooks where id=$1)", r.OrganizationID)
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var organizationUser OrganizationUser
	err := r.DB.QueryRowContext(queryContext, query, applicationId).Scan(
		&organizationUser.OrganizationID,
		&organizationUser.UserID,
		&organizationUser.PermissionBit)
	if err != nil {
		return nil, err
	}

	if (OrganizationUser{}) == organizationUser { // confirm this works
		return nil, err
	}
	return &organizationUser, nil
}

func (r *ApplicationDatabaseStore) GetDatasetUserById(ctx context.Context, applicationId int64, datasetId int64) (*DatasetUser, error) {
	query := fmt.Sprintf("SELECT dataset_id, user_id, role from \"%[1]v\".dataset_user where dataset_id=%[2]v and user_id in (SELECT integration_user_id from \"%[1]v\".webhooks where id=$1)", r.OrganizationID, datasetId)
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var dataserUser DatasetUser
	err := r.DB.QueryRowContext(queryContext, query, applicationId).Scan(
		&dataserUser.DatasetID,
		&dataserUser.UserID,
		&dataserUser.Role)
	if err != nil {
		return nil, err
	}

	if (DatasetUser{}) == dataserUser { // confirm this works
		return nil, err
	}
	return &dataserUser, nil
}

func (r *ApplicationDatabaseStore) GetDatasetUserByUserId(ctx context.Context, userId int64, datasetId int64) (*DatasetUser, error) {
	query := fmt.Sprintf("SELECT dataset_id, user_id, role from \"%[1]v\".dataset_user where dataset_id=%[2]v and user_id=$1)", r.OrganizationID, datasetId)
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var dataserUser DatasetUser
	err := r.DB.QueryRowContext(queryContext, query, userId).Scan(
		&dataserUser.DatasetID,
		&dataserUser.UserID,
		&dataserUser.Role)
	if err != nil {
		return nil, err
	}

	if (DatasetUser{}) == dataserUser { // confirm this works
		return nil, err
	}
	return &dataserUser, nil
}
