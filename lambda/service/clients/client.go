package clients

import (
	"bytes"
	"context"
)

type Client interface {
	Execute(context.Context, bytes.Buffer) ([]byte, error)
}
