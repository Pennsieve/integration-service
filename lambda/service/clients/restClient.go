package clients

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"time"
)

type ApplicationRestClient struct {
	Client         *http.Client
	ApplicationURL string
}

func NewApplicationRestClient(client *http.Client, url string) Client {
	return &ApplicationRestClient{client, url}
}

func (c *ApplicationRestClient) Execute(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	requestDuration := 180 * time.Second
	req, err := http.NewRequest(http.MethodPost, c.ApplicationURL, &b)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	triggerContext, cancel := context.WithTimeout(ctx, requestDuration)
	defer cancel()
	req = req.WithContext(triggerContext)
	resp, err := c.Client.Do(req)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	defer resp.Body.Close()
	s, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return s, err
	}
	return s, nil
}
