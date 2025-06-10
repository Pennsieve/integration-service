package credentialsretriever

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type Retriever interface {
	Run(context.Context) (aws.Credentials, error)
}

type AWSCredentialsRetriever struct {
	AccountId string
	Config    aws.Config
}

func NewAWSCredentialsRetriever(accountId string, cfg aws.Config) Retriever {
	return &AWSCredentialsRetriever{accountId, cfg}
}

func (r *AWSCredentialsRetriever) Run(ctx context.Context) (aws.Credentials, error) {
	log.Println("assuming role ...")

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(os.Getenv("REGION")))
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("failed to load config: %w", err)
	}

	log.Println(cfg)
	stsClient := sts.NewFromConfig(cfg)

	log.Println("getting provisioner account ...")
	provisionerAccountId, err := stsClient.GetCallerIdentity(ctx,
		&sts.GetCallerIdentityInput{})
	if err != nil {
		log.Println("callerIdentity error")
		return aws.Credentials{}, err
	}

	log.Println("getting roleArn ...")
	roleArn := fmt.Sprintf("arn:aws:iam::%s:role/ROLE-%s", r.AccountId, *provisionerAccountId.Account)
	log.Println(roleArn)
	appCreds := stscreds.NewAssumeRoleProvider(stsClient, roleArn)
	credentials, err := appCreds.Retrieve(ctx)
	if err != nil {
		log.Println("appCreds.Retrieve error")
		return aws.Credentials{}, err
	}
	log.Println("done getting creds ...")

	return credentials, nil
}
