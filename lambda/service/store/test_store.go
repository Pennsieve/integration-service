package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type TestDatabaseStore interface {
	InsertOrganizationUser(context.Context, OrganizationUser) (int64, error)
	DeleteOrganizationUser(context.Context, int64, int64) error
	InsertDatasetUser(context.Context, DatasetUser) (int64, error)
	DeleteDatasetUser(context.Context, int64, int64) error
	InsertUser(context.Context, User) (int64, error)
	DeleteUser(context.Context, int64) error
}

type ApplicationTestDatabaseStore struct {
	DB             *sql.DB
	OrganizationID int64
}

func NewApplicationTestDatabaseStore(db *sql.DB, organizationId int64) TestDatabaseStore {
	return &ApplicationTestDatabaseStore{db, organizationId}
}

// utility store methods
func (r *ApplicationTestDatabaseStore) InsertOrganizationUser(ctx context.Context, organizationUser OrganizationUser) (int64, error) {
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

func (r *ApplicationTestDatabaseStore) DeleteOrganizationUser(ctx context.Context, organization_id int64, user_id int64) error {
	query := "DELETE from pennsieve.organization_user WHERE organization_id=$1 and user_id=$2"
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := r.DB.QueryRowContext(queryContext, query, organization_id, user_id)
	if err != nil {
		return err.Err()
	}
	return nil
}

func (r *ApplicationTestDatabaseStore) InsertDatasetUser(ctx context.Context, datasetUser DatasetUser) (int64, error) {
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

func (r *ApplicationTestDatabaseStore) DeleteDatasetUser(ctx context.Context, dataset_id int64, user_id int64) error {
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

func (r *ApplicationTestDatabaseStore) InsertUser(ctx context.Context, user User) (int64, error) {
	var user_id int64
	query := "insert into pennsieve.users (id, email, first_name, last_name, credential, color, url, authy_id, is_super_admin, preferred_org_id, status, node_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING ID"
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := r.DB.QueryRowContext(
		queryContext,
		query,
		user.ID, user.Email, user.FirstName, user.LastName, user.Credential, user.Color,
		user.Url, user.AuthyID, user.IsSuperAdmin, user.PreferredOrgID, user.Status, user.NodeID).Scan(&user_id); err != nil {
		return 0, err
	}
	return user_id, nil
}

func (r *ApplicationTestDatabaseStore) DeleteUser(ctx context.Context, user_id int64) error {
	query := "DELETE from pennsieve.users WHERE id=$1"
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := r.DB.QueryRowContext(queryContext, query, user_id)
	if err != nil {
		return err.Err()
	}
	return nil
}

type User struct {
	ID             int64  `db:"id"`
	Email          string `db:"email"`
	FirstName      string `db:"first_name"`
	LastName       string `db:"last_name"`
	Credential     string `db:"credential"`
	Color          string `db:"color"`
	Url            string `db:"url"`
	AuthyID        int64  `db:"authy_id"`
	IsSuperAdmin   bool   `db:"is_super_admin"`
	PreferredOrgID int64  `db:"preferred_org_id"`
	Status         bool   `db:"status"`
	NodeID         string `db:"node_id"`
}
