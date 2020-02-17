package bearer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Example() {
	client := &http.Client{
		Transport: &Agent{SecretKey: os.Getenv("BEARER_TOKEN")},
	}
	resp, err := client.Get("...")
	if err != nil {
		panic(err)
	}
	fmt.Println("resp", resp)
}

func TestAgent_Config(t *testing.T) {
	sk := os.Getenv("BEARER_TOKEN")
	if sk == "" {
		t.Skip()
	}
	agent := Agent{SecretKey: sk}
	config, err := agent.Config()
	require.NoError(t, err)
	assert.NotNil(t, config)
}

func TestAgent_logRecords(t *testing.T) {
	records := []ReportLog{
		{
			Protocol:        "https",
			Path:            "/sample",
			Hostname:        "api.example.com",
			Method:          "GET",
			StartedAt:       int(time.Now().Add(-80*time.Millisecond).UnixNano() / 1000000),
			EndedAt:         int(time.Now().UnixNano() / 1000000),
			Type:            "REQUEST_END",
			StatusCode:      200,
			URL:             "http://api.example.com/sample",
			RequestHeaders:  map[string]string{"Accept": "application/json"},
			RequestBody:     `{"body":"data"}`,
			ResponseHeaders: map[string]string{"Content-Type": "application/json"},
			ResponseBody:    `{"ok":true}`,
			// instrumentation: ,
		},
	}
	t.Run("unauthenticated", func(t *testing.T) {
		agent := Agent{}
		err := agent.logRecords(records)
		require.Error(t, err)
	})

	sk := os.Getenv("BEARER_TOKEN")
	if sk == "" {
		t.Skip()
	}
	t.Run("authenticated", func(t *testing.T) {
		agent := Agent{SecretKey: sk}
		err := agent.logRecords(records)
		require.NoError(t, err)
	})
}

func TestRoundTrip(t *testing.T) {
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Hello", "World")
		w.Write([]byte("200 OK"))
		w.Write([]byte("Hello World!"))
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	t.Run("unauthenticated", func(t *testing.T) {
		client := &http.Client{
			Transport: &Agent{},
		}

		resp, err := client.Get(ts.URL)
		require.NoError(t, err)
		assert.Equal(t, resp.StatusCode, 200)
	})

	sk := os.Getenv("BEARER_TOKEN")
	if sk == "" {
		t.Skip()
	}
	t.Run("authenticated", func(t *testing.T) {
		client := &http.Client{
			Transport: &Agent{SecretKey: sk},
		}
		resp, err := client.Get(ts.URL + "/test")
		require.NoError(t, err)
		assert.Equal(t, resp.StatusCode, 200)
	})
}