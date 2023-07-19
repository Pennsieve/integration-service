package handler

import "errors"

var ErrApplicationValidation = errors.New("application cannot be triggered")
var ErrRunningTrigger = errors.New("error running trigger")
var ErrUnmarshaling = errors.New("error unmarshaling body")
