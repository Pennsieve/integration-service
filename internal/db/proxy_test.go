package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shaped like an RDS auth token: query-string characters, plus a quote and a
// backslash to exercise quoting.
const tokenLike = `proxy.example.com:5432/?Action=connect&DBUser=x&X-Amz-Date=20260928T120000Z&X-Amz-Signature=a%2Fb'c\d`

func TestProxySettingsFromEnv(t *testing.T) {
	t.Setenv("RDS_PROXY_ENDPOINT", "")
	_, ok := proxySettingsFromEnv()
	assert.False(t, ok, "no endpoint: keep the password connection")

	t.Setenv("RDS_PROXY_ENDPOINT", " proxy.example.com ")
	t.Setenv("RDS_PROXY_USER", "")
	t.Setenv("REGION", "")
	t.Setenv("AWS_REGION", "us-east-1")
	s, ok := proxySettingsFromEnv()
	require.True(t, ok)
	assert.Equal(t, proxySettings{host: "proxy.example.com", user: env + "_rds_proxy_user", region: "us-east-1"}, s)

	t.Setenv("RDS_PROXY_USER", "custom_user")
	t.Setenv("REGION", "eu-west-1")
	s, _ = proxySettingsFromEnv()
	assert.Equal(t, "custom_user", s.user)
	assert.Equal(t, "eu-west-1", s.region)
}

func TestIAMConnectorBuildsATokenPerConnection(t *testing.T) {
	var tokens int
	c := &iamConnector{
		host: "proxy.example.com", port: proxyPort, user: "prod_rds_proxy_user", dbname: "pennsieve_postgres", sslmode: "require",
		token: func(context.Context) (string, error) {
			tokens++
			return fmt.Sprintf("token-%d", tokens), nil
		},
		open: func(context.Context, string) (driver.Conn, error) { return nil, nil },
	}
	for i := 0; i < 3; i++ {
		_, err := c.Connect(context.Background())
		require.NoError(t, err)
	}
	assert.Equal(t, 3, tokens, "a fresh token for every new connection")
	assert.Equal(t, `host='proxy.example.com' port=5432 user='prod_rds_proxy_user' password='t\'k' dbname='pennsieve_postgres' sslmode='require'`, c.dsn(`t'k`))
}

func TestIAMConnectorReportsTokenErrors(t *testing.T) {
	c := &iamConnector{
		token: func(context.Context) (string, error) { return "", errors.New("no credentials") },
		open:  func(context.Context, string) (driver.Conn, error) { t.Fatal("must not connect"); return nil, nil },
	}
	_, err := c.Connect(context.Background())
	assert.ErrorContains(t, err, "building RDS auth token")
}

// Logs in to the test Postgres through the connector and lib/pq with a
// token-shaped password, so the quoting is checked by the real driver.
func TestIAMConnectorPasswordSurvivesLibPQ(t *testing.T) {
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		t.Skip("POSTGRES_HOST not set")
	}
	port, _ := strconv.Atoi(os.Getenv("POSTGRES_PORT"))
	if port == 0 {
		port = 5432
	}
	admin, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
		host, port, os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD")))
	require.NoError(t, err)
	defer admin.Close()

	role := "proxy_test_" + uuid.NewString()[:8]
	_, err = admin.Exec(fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s'`, role, pqEscape(tokenLike)))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec(`DROP ROLE ` + role) })

	c := &iamConnector{
		host: host, port: port, user: role, dbname: "postgres", sslmode: "disable",
		token: func(context.Context) (string, error) { return tokenLike, nil },
		open:  openPQ,
	}
	db := sql.OpenDB(c)
	defer db.Close()
	var user string
	require.NoError(t, db.QueryRow(`SELECT current_user`).Scan(&user))
	assert.Equal(t, role, user)
}

func pqEscape(s string) string {
	out := ""
	for _, r := range s {
		if r == '\'' {
			out += "''"
		} else {
			out += string(r)
		}
	}
	return out
}
