package clients

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/logging"
)

type ComputeRestClient struct {
	Client     *http.Client
	ComputeURL string

	Signer *v4.Signer
	Creds  aws.Credentials
	Region string
	Config aws.Config
}

func NewComputeRestClient(client *http.Client, url string, signer *v4.Signer, creds aws.Credentials, region string, cfg aws.Config) Client {
	return &ComputeRestClient{client, url, signer, creds, region, cfg}
}

func (c *ComputeRestClient) Execute(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	requestDuration := 30 * time.Second
	req, err := http.NewRequest(http.MethodPost, c.ComputeURL, &b)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Compute SHA256 hash of the payload
	sum := sha256.Sum256(b.Bytes())
	payloadHash := hex.EncodeToString(sum[:])

	// sign the request
	err = c.Signer.SignHTTP(ctx, c.Creds, req, payloadHash, "lambda", c.Region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	for k, v := range req.Header {
		fmt.Printf("%s: %s\n", k, v)
	}

	triggerContext, cancel := context.WithTimeout(ctx, requestDuration)
	defer cancel()
	req = req.WithContext(triggerContext)
	resp, err := c.Client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	// Log response status
	log.Printf("Response Status: %s", resp.Status)

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

	req.Header.Set("Content-Type", "application/json")

	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	retrievalContext, cancel := context.WithTimeout(ctx, requestDuration)
	defer cancel()
	req = req.WithContext(retrievalContext)

	fmt.Println("AccessKey:", c.Creds.AccessKeyID)
	fmt.Println("SessionToken present:", c.Creds.SessionToken != "")
	fmt.Println("Region:", c.Region)

	// test if you can list buckets
	client := s3.NewFromConfig(c.Config, func(o *s3.Options) {
		o.Credentials = credentials.NewStaticCredentialsProvider(c.Creds.AccessKeyID, c.Creds.SecretAccessKey, c.Creds.SessionToken)
	})
	buckets, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	for _, b := range buckets.Buckets {
		fmt.Println(*b.Name)
	}

	const emptyStringSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	// sign the request
	err = c.Signer.SignHTTP(ctx, c.Creds, req, emptyStringSHA256, "lambda", c.Region, time.Now(),
		func(o *v4.SignerOptions) {
			o.LogSigning = true
			o.Logger = logging.NewStandardLogger(os.Stderr)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	for k, v := range req.Header {
		fmt.Printf("%s: %s\n", k, v)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	// Log response status
	log.Printf("Response Status: %s", resp.Status)

	defer resp.Body.Close()
	s, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return s, err
	}
	return s, nil
}
