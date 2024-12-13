package clients

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"time"
)

type ComputeRestClient struct {
	Client     *http.Client
	ComputeURL string
}

func NewComputeRestClient(client *http.Client, url string) Client {
	return &ComputeRestClient{client, url}
}

func (c *ComputeRestClient) Execute(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	requestDuration := 30 * time.Second
	req, err := http.NewRequest(http.MethodPost, c.ComputeURL, &b)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	triggerContext, cancel := context.WithTimeout(ctx, requestDuration)
	defer cancel()
	req = req.WithContext(triggerContext)
	resp, err := c.Client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	defer resp.Body.Close()
	s, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return s, err
	}
	return s, nil
}

func (c *ComputeRestClient) Retrieve(ctx context.Context, params map[string]string) ([]byte, error) {
	requestDuration := 30 * time.Second
	req, err := http.NewRequest(http.MethodGet, c.ComputeURL, nil)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	retrievalContext, cancel := context.WithTimeout(ctx, requestDuration)
	defer cancel()
	req = req.WithContext(retrievalContext)
	resp, err := c.Client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	defer resp.Body.Close()
	s, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return s, err
	}
	return s, nil
}
