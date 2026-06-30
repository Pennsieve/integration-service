package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

var (
	ssmClient  *ssm.Client
	AwsOnce    sync.Once
	awsInitErr error
)

func InitAWS(ctx context.Context) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		awsInitErr = fmt.Errorf("unable to load SDK config: %w", err)
		return
	}
	ssmClient = ssm.NewFromConfig(cfg)
}

func GetSSMParam(ctx context.Context, name string, decrypt bool) (string, error) {
	if awsInitErr != nil {
		return "", awsInitErr
	}

	if ssmClient == nil {
		return "", fmt.Errorf("uninitialized SSM client")
	}

	input := &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: &decrypt,
	}

	output, err := ssmClient.GetParameter(ctx, input)
	if err != nil {
		return "", fmt.Errorf("unable to fetch SSM parameter %s: %w", name, err)
	}
	return *output.Parameter.Value, nil
}
