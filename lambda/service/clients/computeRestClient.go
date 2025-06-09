package clients

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

type ComputeRestClient struct {
	Client     *http.Client
	ComputeURL string
	Signer     *v4.Signer
	Creds      aws.Credentials
	Region     string
}

func NewComputeRestClient(client *http.Client, url string, signer *v4.Signer, creds aws.Credentials, region string) Client {
	return &ComputeRestClient{client, url, signer, creds, region}
}

func (c *ComputeRestClient) Execute(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	requestDuration := 30 * time.Second
	req, err := http.NewRequest(http.MethodPost, c.ComputeURL, &b)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	// sign the request
	err = c.Signer.SignHTTP(ctx, c.Creds, req, "", "compute-gateway", c.Region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
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

	// sign the request
	err = c.Signer.SignHTTP(ctx, c.Creds, req, "", "compute-gateway", c.Region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

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
