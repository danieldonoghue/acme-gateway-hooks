package azure

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

type Client struct {
	httpClient *http.Client
	token      string
	tokenExp   time.Time
	tenantID   string
	clientID   string
	secret     string
	certPath   string
	certPass   string
	baseURL    string
}

type RecordSetResponse struct {
	ID    string
	Name  string
	Value string
}

type TXTRecord struct {
	Value []string `json:"value"`
}

type RecordSetProperties struct {
	TTL        int32       `json:"TTL"`
	TxtRecords []TXTRecord `json:"TXTRecords"`
}

type RecordSetPayload struct {
	Properties RecordSetProperties `json:"properties"`
}

type RecordSetResp struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Properties RecordSetProperties `json:"properties"`
}

func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	baseURL := "https://management.azure.com"
	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}

	tenantID := cfg.TenantID
	if tenantID == "" {
		discovered, err := discoverTenantID(ctx, baseURL, cfg.SubscriptionID)
		if err != nil {
			return nil, fmt.Errorf("AZURE_TENANT_ID not set and auto-discovery failed: %w", err)
		}
		tenantID = discovered
	}

	client := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		tenantID:   tenantID,
		clientID:   cfg.ClientID,
		secret:     cfg.ClientSecret,
		certPath:   cfg.ClientCertPath,
		certPass:   cfg.ClientCertPassword,
		baseURL:    baseURL,
	}

	if err := client.refreshToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to obtain Azure token: %w", err)
	}

	return client, nil
}

func (c *Client) refreshToken(ctx context.Context) error {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.tenantID)
	if c.baseURL != "https://management.azure.com" {
		tokenURL = fmt.Sprintf("%s/%s/oauth2/v2.0/token", c.baseURL, c.tenantID)
	}

	var data url.Values
	if c.secret != "" {
		data = url.Values{
			"client_id":     {c.clientID},
			"client_secret": {c.secret},
			"scope":         {"https://management.azure.com/.default"},
			"grant_type":    {"client_credentials"},
		}
	} else if c.certPath != "" {
		assertion, err := c.createJWT()
		if err != nil {
			return fmt.Errorf("failed to create JWT: %w", err)
		}
		data = url.Values{
			"client_id":             {c.clientID},
			"client_assertion":      {assertion},
			"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
			"scope":                 {"https://management.azure.com/.default"},
			"grant_type":            {"client_credentials"},
		}
	} else {
		return fmt.Errorf("no authentication method configured")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	c.token = tokenResp.AccessToken
	c.tokenExp = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

func (c *Client) ensureValidToken(ctx context.Context) error {
	if time.Now().Before(c.tokenExp.Add(-1 * time.Minute)) {
		return nil
	}
	return c.refreshToken(ctx)
}

func (c *Client) createJWT() (string, error) {
	certData, err := os.ReadFile(c.certPath)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate: %w", err)
	}

	var privateKey *rsa.PrivateKey

	// Try PKCS12 first (with or without password)
	if c.certPass != "" || isPKCS12(certData) {
		priv, _, err := pkcs12.Decode(certData, c.certPass)
		if err == nil {
			var ok bool
			privateKey, ok = priv.(*rsa.PrivateKey)
			if !ok {
				return "", fmt.Errorf("certificate does not contain RSA private key")
			}
		}
	}

	// If PKCS12 didn't work or wasn't tried, try PEM then raw DER
	if privateKey == nil {
		var derBytes []byte
		rest := certData
		for {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}
			if strings.Contains(block.Type, "PRIVATE KEY") {
				derBytes = block.Bytes
				break
			}
		}
		if derBytes == nil {
			derBytes = certData
		}

		priv, err := x509.ParsePKCS8PrivateKey(derBytes)
		if err != nil {
			priv, err = x509.ParsePKCS1PrivateKey(derBytes)
			if err != nil {
				return "", fmt.Errorf("failed to parse private key (tried PKCS12, PKCS8, PKCS1): %w", err)
			}
		}
		var ok bool
		privateKey, ok = priv.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("certificate does not contain RSA private key")
		}
	}

	now := time.Now().Unix()
	exp := now + 3600

	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}

	claims := map[string]interface{}{
		"sub": c.clientID,
		"iss": c.clientID,
		"aud": fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.tenantID),
		"exp": exp,
		"iat": now,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	message := []byte(headerB64 + "." + claimsB64)

	hash := sha256.Sum256(message)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)

	return headerB64 + "." + claimsB64 + "." + signatureB64, nil
}

// isPKCS12 checks if data looks like PKCS12 by magic bytes
func isPKCS12(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	// PKCS12 starts with SEQUENCE tag (0x30) or can start with 0xfe 0xff (UTF-16 BOM)
	return (data[0] == 0x30 && len(data) > 3 && data[1] > 0x80) || (data[0] == 0xfe && data[1] == 0xff)
}

func (c *Client) getTXTRecordSet(ctx context.Context, subscriptionID, resourceGroup, zoneName, recordName string) (*RecordSetResp, error) {
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnszones/%s/TXT/%s?api-version=2018-05-01", c.baseURL, subscriptionID, resourceGroup, zoneName, recordName)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure API returned status %d: %s", resp.StatusCode, string(body))
	}

	var respData RecordSetResp
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &respData, nil
}

func (c *Client) CreateTXTRecord(ctx context.Context, subscriptionID, resourceGroup, zoneName, recordName, txtValue string, ttl int32) (*RecordSetResponse, error) {
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	// Get existing values to avoid overwriting
	existing, err := c.ListTXTRecords(ctx, subscriptionID, resourceGroup, zoneName, recordName)
	if err != nil {
		return nil, err
	}

	// Merge with existing values (deduplicate)
	valueSet := make(map[string]bool)
	for _, v := range existing {
		valueSet[v] = true
	}
	valueSet[txtValue] = true

	var values []string
	for v := range valueSet {
		values = append(values, v)
	}

	// Create TXT records array — each value is a separate TXT RR
	var txtRecords []TXTRecord
	for _, v := range values {
		txtRecords = append(txtRecords, TXTRecord{Value: []string{v}})
	}

	payload := RecordSetPayload{
		Properties: RecordSetProperties{
			TTL:        ttl,
			TxtRecords: txtRecords,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnszones/%s/TXT/%s?api-version=2018-05-01", c.baseURL, subscriptionID, resourceGroup, zoneName, recordName)
	req, err := http.NewRequestWithContext(ctx, "PUT", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure API returned status %d: %s", resp.StatusCode, string(body))
	}

	var respData RecordSetResp
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &RecordSetResponse{
		ID:    respData.ID,
		Name:  recordName,
		Value: txtValue,
	}, nil
}

func (c *Client) ListTXTRecords(ctx context.Context, subscriptionID, resourceGroup, zoneName, recordName string) ([]string, error) {
	rs, err := c.getTXTRecordSet(ctx, subscriptionID, resourceGroup, zoneName, recordName)
	if err != nil {
		return nil, err
	}
	if rs == nil {
		return []string{}, nil
	}

	var values []string
	for _, txtRec := range rs.Properties.TxtRecords {
		values = append(values, strings.Join(txtRec.Value, ""))
	}

	return values, nil
}

func (c *Client) DeleteTXTRecord(ctx context.Context, subscriptionID, resourceGroup, zoneName, recordName, value string) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}

	rs, err := c.getTXTRecordSet(ctx, subscriptionID, resourceGroup, zoneName, recordName)
	if err != nil {
		return err
	}
	if rs == nil {
		return nil
	}

	var remaining []string
	for _, txtRec := range rs.Properties.TxtRecords {
		v := strings.Join(txtRec.Value, "")
		if v != value {
			remaining = append(remaining, v)
		}
	}

	// If no values left, delete the entire recordset
	if len(remaining) == 0 {
		endpoint := fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnszones/%s/TXT/%s?api-version=2018-05-01", c.baseURL, subscriptionID, resourceGroup, zoneName, recordName)
		req, err := http.NewRequestWithContext(ctx, "DELETE", endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		// 404 is acceptable if record already deleted
		if resp.StatusCode == 404 {
			return nil
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("Azure API returned status %d: %s", resp.StatusCode, string(body))
		}
		return nil
	}

	// Otherwise, update the recordset with remaining values
	var remainingRecords []TXTRecord
	for _, v := range remaining {
		remainingRecords = append(remainingRecords, TXTRecord{Value: []string{v}})
	}

	payload := RecordSetPayload{
		Properties: RecordSetProperties{
			TTL:        rs.Properties.TTL,
			TxtRecords: remainingRecords,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnszones/%s/TXT/%s?api-version=2018-05-01", c.baseURL, subscriptionID, resourceGroup, zoneName, recordName)
	req, err := http.NewRequestWithContext(ctx, "PUT", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Azure API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
