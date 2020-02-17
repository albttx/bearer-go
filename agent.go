package bearer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
)

func (a *Agent) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := a.transport().RoundTrip(req)
	end := time.Now()

	if a.SecretKey != "" {
		record := ReportLog{
			Protocol:        req.URL.Scheme,
			Path:            req.URL.Path,
			Hostname:        req.URL.Hostname(),
			Method:          req.Method,
			StartedAt:       int(start.UnixNano() / 1000000),
			EndedAt:         int(end.UnixNano() / 1000000),
			Type:            "REQUEST_END",
			StatusCode:      resp.StatusCode,
			URL:             req.URL.String(),
			RequestHeaders:  goHeadersToBearerHeaders(req.Header),
			RequestBody:     "", // FIXME: log body
			ResponseHeaders: goHeadersToBearerHeaders(resp.Header),
			ResponseBody:    "", // FIXME: log body
		}
		if err := a.logRecords([]ReportLog{record}); err != nil {
			a.logger().Warn("log records", zap.Error(err))
		}
	}
	return resp, err
}

func goHeadersToBearerHeaders(input http.Header) map[string]string {
	ret := map[string]string{}
	for key, values := range input {
		// bearer headers only support one value per key
		// so we take the first one and ignore the other ones
		ret[key] = values[0]
	}
	return ret
}

func (a Agent) Config() (*Config, error) {
	req, err := http.NewRequest("GET", "https://config.bearer.sh/config", nil)
	if err != nil {
		return nil, fmt.Errorf("create config request: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", a.SecretKey)

	ret, err := a.transport().RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer ret.Body.Close()

	// parse body
	body, err := ioutil.ReadAll(ret.Body)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (a Agent) logger() *zap.Logger {
	if a.Logger != nil {
		return a.Logger
	}
	return zap.NewNop()
}

func (a Agent) transport() http.RoundTripper {
	if a.Transport != nil {
		return a.Transport
	}
	return http.DefaultTransport
}

func (a Agent) logRecords(records []ReportLog) error {
	if len(records) < 1 {
		return nil
	}

	type logsRequest struct {
		SecretKey string `json:"secretKey"`
		Runtime   struct {
			Type    string `json:"type"`
			Version string `json:"version"`
		} `json:"runtime"`
		Agent struct {
			Type     string `json:"type"`
			Version  string `json:"version"`
			LogLevel string `json:"log_level"`
			// FIXME: Config
		} `json:"agent"`
		Logs []ReportLog `json:"logs"`
	}
	input := logsRequest{SecretKey: a.SecretKey, Logs: records}
	input.Runtime.Type = "go"
	input.Runtime.Version = runtime.Version()
	input.Agent.Type = "bearer-go"
	input.Agent.Version = Version
	input.Agent.LogLevel = "ALL"

	inputJson, err := json.Marshal(input)
	if err != nil {
		return err
	}
	reqBody := ioutil.NopCloser(strings.NewReader(string(inputJson)))
	req, err := http.NewRequest("POST", "https://agent.bearer.sh/logs", reqBody)
	if err != nil {
		return fmt.Errorf("create logs request: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	ret, err := a.transport().RoundTrip(req)
	if err != nil {
		return fmt.Errorf("perform logs request: %w", err)
	}
	defer ret.Body.Close()
	switch ret.StatusCode {
	case 200:
		return nil
	default:
		/*
			body, err := ioutil.ReadAll(ret.Body)
			if err != nil {
				return fmt.Errorf("read logs body: %w", err)
			}
			var resp struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return fmt.Errorf("parse logs response: %w", err)
			}
		*/

		return fmt.Errorf("unsupported status code: %d", ret.StatusCode)
	}
}