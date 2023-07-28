package repository

import "time"

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
