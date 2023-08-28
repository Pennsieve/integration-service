package store

import (
	"time"

	"github.com/pennsieve/pennsieve-go-core/pkg/models/pgdb"
)

type Application struct {
	ID                int64     `db:"id"`
	Name              string    `db:"name"`
	Description       string    `db:"description"`
	DisplayName       string    `db:"display_name"`
	URL               string    `db:"api_url"`
	Secret            string    `db:"secret"`
	IsDisabled        bool      `db:"is_disabled"`
	IsPrivate         bool      `db:"is_private"`
	IsDefault         bool      `db:"is_default"`
	CreatedAt         time.Time `db:"created_at"`
	CreatedBy         int64     `db:"created_by"`
	IntegrationUserID int64     `db:"integration_user_id"`
	HasAccess         bool      `db:"has_access"`
}

type OrganizationUser struct {
	OrganizationID int64 `db:"organization_id"`
	UserID         int64 `db:"user_id"`
	// Role not included: permission_bit used in claims, consistency?
	PermissionBit pgdb.DbPermission
}

type DatasetUser struct {
	DatasetID int64  `db:"dataset_id"`
	UserID    int64  `db:"user_id"`
	Role      string `db:"role"`
}
