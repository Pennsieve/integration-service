package clients

import "bytes"

type Client interface {
	Execute(b bytes.Buffer) ([]byte, error)
}
