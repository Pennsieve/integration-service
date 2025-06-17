package clients

import (
	"bytes"
	"context"
)

type Client interface {
	Execute(context.Context, bytes.Buffer) ([]byte, error)
	Retrieve(context.Context, map[string]string) ([]byte, error)

	// Legacy
	ExecuteLegacy(context.Context, bytes.Buffer) ([]byte, error)
	RetrieveLegacy(context.Context, map[string]string) ([]byte, error)
}
