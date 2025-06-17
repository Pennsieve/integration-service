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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/logging"
	"github.com/pennsieve/integration-service/service/credentials_retriever"
)

type ComputeRestClient struct {
	Client     *http.Client
	ComputeURL string

	Region    string
	Config    aws.Config
	AccountId string
}

func NewComputeRestClient(client *http.Client, url string, region string, cfg aws.Config, accountId string) Client {
	return &ComputeRestClient{client, url, region, cfg, accountId}
}

func (c *ComputeRestClient) Execute(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	log.Println("Starting execute: IAM")
	requestDuration := 30 * time.Second
	req, err := http.NewRequest(http.MethodPost, c.ComputeURL, &b)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	retriever := credentials_retriever.NewAWSCredentialsRetriever(c.AccountId, c.Config)
	creds, err := retriever.Run(ctx)
	if err != nil {
		return nil, err
	}

	// reload config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Create STS client
	newStsClient := sts.NewFromConfig(cfg, func(o *sts.Options) {
		o.Credentials = credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)
	})

	// Call GetCallerIdentity
	caller, err := newStsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}

	// Print the ARN of the assumed role
	fmt.Printf("ARN of assumed role: %s\n", *caller.Arn)

	// Compute SHA256 hash of the payload
	sum := sha256.Sum256(b.Bytes())
	payloadHash := hex.EncodeToString(sum[:])

	// sign the request
	signer := v4.NewSigner()
	err = signer.SignHTTP(ctx, creds, req, payloadHash, "lambda", c.Region, time.Now())
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
	log.Println("Starting retrieve: IAM")
	requestDuration := 30 * time.Second
	log.Println("url: ", c.ComputeURL)
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

	retriever := credentials_retriever.NewAWSCredentialsRetriever(c.AccountId, c.Config)
	creds, err := retriever.Run(ctx)
	if err != nil {
		return nil, err
	}

	// reload config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Create STS client
	newStsClient := sts.NewFromConfig(cfg, func(o *sts.Options) {
		o.Credentials = credentials.NewStaticCredentialsProvider(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)
	})

	// Call GetCallerIdentity
	caller, err := newStsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}

	// Print the ARN of the assumed role
	fmt.Printf("ARN of assumed role: %s\n", *caller.Arn)

	const emptyStringSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	// sign the request
	signer := v4.NewSigner()
	err = signer.SignHTTP(ctx, creds, req, emptyStringSHA256, "lambda", c.Region, time.Now(),
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

// Legacy non-IAM implementations for fallback
func (c *ComputeRestClient) ExecuteLegacy(ctx context.Context, b bytes.Buffer) ([]byte, error) {
	log.Println("Starting execute: LEGACY")
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

	log.Printf("Response Status: %s", resp.Status)
	defer resp.Body.Close()
	s, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return s, err
	}
	return s, nil
}

func (c *ComputeRestClient) RetrieveLegacy(ctx context.Context, params map[string]string) ([]byte, error) {
	log.Println("Starting retrieve: LEGACY")
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

	log.Printf("Response Status: %s", resp.Status)

	defer resp.Body.Close()
	s, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return s, err
	}
	return s, nil
}
