package repository

type Application struct {
	ID         int64  `db:"id"`
	Name       string `db:"name"`
	URL        string `db:"api_url"`
	IsDisabled bool   `db:"is_disabled"`
}
