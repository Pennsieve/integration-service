package clients

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type ApplicationRestClient struct {
	Client         *http.Client
	ApplicationURL string
	Logger         *slog.Logger
}

func NewApplicationRestClient(client *http.Client, url string, logger *slog.Logger) Client {
	return &ApplicationRestClient{client, url, logger}
}

func (c *ApplicationRestClient) Execute(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	requestDuration := 30 * time.Second
	req, err := http.NewRequest(http.MethodPost, c.ApplicationURL, &b)
	if err != nil {
		c.Logger.ErrorContext(ctx, err.Error())
		return nil, err
	}

	triggerContext, cancel := context.WithTimeout(ctx, requestDuration)
	defer cancel()
	req = req.WithContext(triggerContext)
	resp, err := c.Client.Do(req)
	if err != nil {
		c.Logger.ErrorContext(ctx, err.Error())
		return nil, err
	}

	defer resp.Body.Close()
	s, err := io.ReadAll(resp.Body)
	if err != nil {
		c.Logger.ErrorContext(ctx, err.Error())
		return s, err
	}
	return s, nil
}
