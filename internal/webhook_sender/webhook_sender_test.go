package webhook_sender

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendWebhookWithRetry_SuccessNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := sendWebhookWithRetry(context.Background(), srv.URL, []byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "a 2xx should not retry")
}

func TestSendWebhookWithRetry_Non2xxReportsStatus(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := sendWebhookWithRetry(context.Background(), srv.URL, []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-2xx status 500")
	assert.NotContains(t, err.Error(), "%!w", "error must not wrap a nil")
	assert.Equal(t, int32(maxRetries), atomic.LoadInt32(&calls), "non-2xx should exhaust retries")
}

func TestSendWebhookWithRetry_TransportErrorReported(t *testing.T) {
	err := sendWebhookWithRetry(context.Background(), "http://127.0.0.1:0", []byte(`{}`))
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "after 3 attempts"))
	assert.NotContains(t, err.Error(), "%!w", "error must not wrap a nil")
}
