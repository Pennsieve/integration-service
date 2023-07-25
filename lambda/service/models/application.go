package models

type Application struct {
	ID         int64
	Name       string
	URL        string
	IsActive   bool
	IsInternal bool
}
