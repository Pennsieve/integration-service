package handler

import "errors"

var ErrApplicationValidation = errors.New("application cannot be triggered")
var ErrRunningTrigger = errors.New("error running trigger")
var ErrUnmarshaling = errors.New("error unmarshaling body")
var ErrUnsupportedRoute = errors.New("unsupported route")
var ErrDatabaseConnection = errors.New("error connecting to database")
var ErrNoRecordsFound = errors.New("error no records found")
var ErrUnsupportedPath = errors.New("unsupported path")
var ErrUnauthorized = errors.New("not authorized to perform this action")
