package credentials_retriever

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
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

func (c *AWSCredentialsRetriever) Run(ctx context.Context) (aws.Credentials, error) {
	// get credentials
	stsClient := sts.NewFromConfig(c.Config)

	log.Println("getting provisioner account ...")
	provisionerAccountId, err := stsClient.GetCallerIdentity(ctx,
		&sts.GetCallerIdentityInput{})
	if err != nil {
		log.Println("callerIdentity error: ", err.Error())
		return aws.Credentials{}, err
	}
	fmt.Printf("ARN of provisioner: %s\n", *provisionerAccountId.Arn)

	log.Println("getting roleArn ...")
	roleArn := fmt.Sprintf("arn:aws:iam::%s:role/ROLE-%s", c.AccountId, *provisionerAccountId.Account)

	appCreds := stscreds.NewAssumeRoleProvider(stsClient, roleArn)
	creds, err := appCreds.Retrieve(ctx)
	if err != nil {
		log.Println("appCreds.Retrieve error: ", err.Error())
		return aws.Credentials{}, err
	}
	log.Println("done getting creds ...")

	return creds, nil
}
