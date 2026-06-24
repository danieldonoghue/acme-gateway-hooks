package excedo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	SuccessCode       = 1000
	SuccessNoMsgCode  = 1300
	NotFoundCode      = 2303
	defaultTimeout    = 10 * time.Second
	defaultMaxRetries = 4
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	maxRetries int
	backoff    time.Duration
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: defaultTimeout},
		maxRetries: defaultMaxRetries,
		backoff:    250 * time.Millisecond,
	}
}

func (c *Client) Login(ctx context.Context) (string, error) {
	endpoint := fmt.Sprintf("%s/authenticate/login/%s", c.baseURL, url.PathEscape(c.token))
	resp, err := c.doJSONRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	var authResp AuthResponse
	if err := decodeJSONStrict(resp, &authResp); err != nil {
		return "", fmt.Errorf("decode auth response: %w", err)
	}
	token := strings.TrimSpace(authResp.Token)
	if token == "" {
		token = authResp.ParametersToken()
	}
	if !isSuccessCode(authResp.Code) || token == "" {
		return "", fmt.Errorf("authentication failed: code=%d", authResp.Code)
	}
	return token, nil
}

func isSuccessCode(code int) bool {
	return code == SuccessCode || code == SuccessNoMsgCode
}

func (c *Client) GetRecords(ctx context.Context, sessionToken, domainName string) (*GetRecordsResponse, error) {
	u := fmt.Sprintf("%s/dns/getrecords/%s?domainname=%s", c.baseURL, url.PathEscape(sessionToken), url.QueryEscape(domainName))
	resp, err := c.doJSONRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	var recordsResp GetRecordsResponse
	if err := decodeJSONStrict(resp, &recordsResp); err != nil {
		return nil, fmt.Errorf("decode getrecords response: %w", err)
	}
	return &recordsResp, nil
}

func (c *Client) AddTXTRecord(ctx context.Context, sessionToken, domainName, recordName, value string, ttl int) (*AddRecordResponse, error) {
	form := url.Values{}
	form.Set("type", "TXT")
	form.Set("name", recordName)
	form.Set("content", value)
	form.Set("ttl", fmt.Sprintf("%d", ttl))
	form.Set("domainname", domainName)

	u := fmt.Sprintf("%s/dns/addrecord/%s", c.baseURL, url.PathEscape(sessionToken))
	resp, err := c.doJSONRequest(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	var addResp AddRecordResponse
	if err := decodeJSONStrict(resp, &addResp); err != nil {
		return nil, fmt.Errorf("decode addrecord response: %w", err)
	}
	return &addResp, nil
}

func (c *Client) DeleteRecord(ctx context.Context, sessionToken, domainName, recordID string) (*DeleteRecordResponse, error) {
	form := url.Values{}
	form.Set("recordid", recordID)
	form.Set("domainname", domainName)

	u := fmt.Sprintf("%s/dns/deleterecord/%s", c.baseURL, url.PathEscape(sessionToken))
	resp, err := c.doJSONRequest(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	var delResp DeleteRecordResponse
	if err := decodeJSONStrict(resp, &delResp); err != nil {
		return nil, fmt.Errorf("decode deleterecord response: %w", err)
	}
	return &delResp, nil
}

func (c *Client) doJSONRequest(ctx context.Context, method, endpoint string, body io.Reader) ([]byte, error) {
	var payload []byte
	if body != nil {
		raw, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		payload = raw
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		reqBody := io.Reader(nil)
		if payload != nil {
			reqBody = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
		if err != nil {
			return nil, err
		}
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if !isRetryableStatus(0) || attempt == c.maxRetries {
				break
			}
			sleepWithBackoff(ctx, c.backoff, attempt)
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt == c.maxRetries {
				break
			}
			sleepWithBackoff(ctx, c.backoff, attempt)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}

		lastErr = fmt.Errorf("http %d from %s", resp.StatusCode, endpoint)
		if !isRetryableStatus(resp.StatusCode) || attempt == c.maxRetries {
			break
		}
		sleepWithBackoff(ctx, c.backoff, attempt)
	}

	return nil, lastErr
}

func decodeJSONStrict(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("unexpected trailing JSON content")
	}
	return nil
}

func sleepWithBackoff(ctx context.Context, base time.Duration, attempt int) {
	d := base * time.Duration(1<<attempt)
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func isRetryableStatus(status int) bool {
	if status == 0 {
		return true
	}
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}
