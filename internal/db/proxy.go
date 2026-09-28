package db

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/lib/pq"
)

// Connecting through the RDS proxy with IAM authentication, as the platform's
// other Lambdas do (pennsieve-go-core pgdb.ConnectRDS), instead of a
// long-lived database password. Selected by setting RDS_PROXY_ENDPOINT.
//
// Unlike pgdb.ConnectRDS, the token is built per connection: this package
// keeps its pool across invocations, and an IAM auth token is only valid for
// 15 minutes, so a token built once would fail for any connection the pool
// opens later.

const proxyPort = 5432

// proxySettings is read from the environment: RDS_PROXY_ENDPOINT (required),
// RDS_PROXY_USER (default <ENV>_rds_proxy_user) and the region
// (REGION, else AWS_REGION, which Lambda sets).
type proxySettings struct {
	host   string
	user   string
	region string
}

func proxySettingsFromEnv() (proxySettings, bool) {
	host := strings.TrimSpace(os.Getenv("RDS_PROXY_ENDPOINT"))
	if host == "" {
		return proxySettings{}, false
	}
	user := os.Getenv("RDS_PROXY_USER")
	if user == "" {
		user = fmt.Sprintf("%s_rds_proxy_user", env)
	}
	region := os.Getenv("REGION")
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	return proxySettings{host: host, user: user, region: region}, true
}

type tokenFunc func(ctx context.Context) (string, error)

// iamConnector is a driver.Connector that authenticates every new connection
// with a fresh IAM auth token.
type iamConnector struct {
	host    string
	port    int
	user    string
	dbname  string
	sslmode string
	token   tokenFunc
	open    func(ctx context.Context, dsn string) (driver.Conn, error)
}

func newIAMConnector(ctx context.Context, settings proxySettings, dbname string) (*iamConnector, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(settings.region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	return &iamConnector{
		host:    settings.host,
		port:    proxyPort,
		user:    settings.user,
		dbname:  dbname,
		sslmode: "require",
		token:   iamToken(settings, cfg.Credentials),
		open:    openPQ,
	}, nil
}

func iamToken(settings proxySettings, creds awssdk.CredentialsProvider) tokenFunc {
	endpoint := net.JoinHostPort(settings.host, strconv.Itoa(proxyPort))
	return func(ctx context.Context) (string, error) {
		return auth.BuildAuthToken(ctx, endpoint, settings.region, settings.user, creds)
	}
}

func openPQ(ctx context.Context, dsn string) (driver.Conn, error) {
	c, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, err
	}
	return c.Connect(ctx)
}

func (c *iamConnector) Connect(ctx context.Context) (driver.Conn, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, fmt.Errorf("building RDS auth token: %w", err)
	}
	return c.open(ctx, c.dsn(token))
}

func (c *iamConnector) Driver() driver.Driver { return &pq.Driver{} }

// dsn quotes every value: the token contains characters (&, =, %) that are
// only safe inside quotes in a key/value connection string.
func (c *iamConnector) dsn(token string) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		quoteDSN(c.host), c.port, quoteDSN(c.user), quoteDSN(token), quoteDSN(c.dbname), quoteDSN(c.sslmode))
}

func quoteDSN(v string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
}
