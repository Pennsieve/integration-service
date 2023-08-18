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
	GetOrganizationUserById(context.Context, int64) (*OrganizationUser, error)
	GetDatasetUserById(context.Context, int64, int64) (*DatasetUser, error)
	GetDatasetUserByUserId(context.Context, int64, int64) (*DatasetUser, error)
	// Utility methods
	InsertOrganizationUser(context.Context, OrganizationUser) (int64, error)
	DeleteOrganizationUser(context.Context, int64, int64) error
	InsertDatasetUser(context.Context, DatasetUser) (int64, error)
	DeleteDatasetUser(context.Context, int64, int64) error
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

func (r *ApplicationDatabaseStore) GetOrganizationUserById(ctx context.Context, applicationId int64) (*OrganizationUser, error) {
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

	return &dataserUser, nil
}

func (r *ApplicationDatabaseStore) GetDatasetUserByUserId(ctx context.Context, userId int64, datasetId int64) (*DatasetUser, error) {
	query := fmt.Sprintf("SELECT dataset_id, user_id, role from \"%[1]v\".dataset_user where dataset_id=%[2]v and user_id=$1", r.OrganizationID, datasetId)
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

	return &dataserUser, nil
}

// utility store methods
func (r *ApplicationDatabaseStore) InsertOrganizationUser(ctx context.Context, organizationUser OrganizationUser) (int64, error) {
	var organization_id int64
	query := "insert into pennsieve.organization_user (organization_id,user_id,permission_bit) VALUES ($1,$2,$3) RETURNING ORGANIZATION_ID"
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := r.DB.QueryRowContext(
		queryContext,
		query,
		organizationUser.OrganizationID,
		organizationUser.UserID,
		organizationUser.PermissionBit).Scan(&organization_id); err != nil {
		return 0, err
	}
	return organization_id, nil
}

func (r *ApplicationDatabaseStore) DeleteOrganizationUser(ctx context.Context, organization_id int64, user_id int64) error {
	query := "DELETE from pennsieve.organization_user WHERE organization_id=$1 and user_id=$2"
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := r.DB.QueryRowContext(queryContext, query, organization_id, user_id)
	if err != nil {
		return err.Err()
	}
	return nil
}

func (r *ApplicationDatabaseStore) InsertDatasetUser(ctx context.Context, datasetUser DatasetUser) (int64, error) {
	var dataset_id int64
	query := fmt.Sprintf("insert into \"%v\".dataset_user (dataset_id,user_id,role) VALUES ($1,$2,$3) RETURNING DATASET_ID",
		r.OrganizationID)
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := r.DB.QueryRowContext(
		queryContext,
		query,
		datasetUser.DatasetID,
		datasetUser.UserID,
		datasetUser.Role).Scan(&dataset_id); err != nil {
		return 0, err
	}
	return dataset_id, nil
}

func (r *ApplicationDatabaseStore) DeleteDatasetUser(ctx context.Context, dataset_id int64, user_id int64) error {
	query := fmt.Sprintf("DELETE from \"%v\".dataset_user WHERE dataset_id=$1 and user_id=$2",
		r.OrganizationID)
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := r.DB.QueryRowContext(queryContext, query, dataset_id, user_id)
	if err != nil {
		return err.Err()
	}
	return nil
}
