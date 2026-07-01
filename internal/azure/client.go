package azure

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
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
	token    string
	tokenExp time.Time
	tenantID string
	clientID string
	secret   string
	certPath string
	certPass string
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
	client := &Client{
		tenantID: cfg.TenantID,
		clientID: cfg.ClientID,
		secret:   cfg.ClientSecret,
		certPath: cfg.ClientCertPath,
		certPass: cfg.ClientCertPassword,
	}

	if err := client.refreshToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to obtain Azure token: %w", err)
	}

	return client, nil
}

func (c *Client) refreshToken(ctx context.Context) error {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.tenantID)

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

	resp, err := http.DefaultClient.Do(req)
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
	if c.certPass != "" {
		priv, _, err := pkcs12.Decode(certData, c.certPass)
		if err != nil {
			return "", fmt.Errorf("failed to decode PKCS12 certificate: %w", err)
		}
		var ok bool
		privateKey, ok = priv.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("certificate does not contain RSA private key")
		}
	} else {
		priv, err := x509.ParsePKCS8PrivateKey(certData)
		if err != nil {
			priv, err = x509.ParsePKCS1PrivateKey(certData)
			if err != nil {
				return "", fmt.Errorf("failed to parse private key: %w", err)
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
		"kid": c.clientID,
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
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, 0, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)

	return headerB64 + "." + claimsB64 + "." + signatureB64, nil
}

func (c *Client) CreateTXTRecord(ctx context.Context, subscriptionID, resourceGroup, zoneName, recordName, txtValue string, ttl int32) (*RecordSetResponse, error) {
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	payload := RecordSetPayload{
		Properties: RecordSetProperties{
			TTL: ttl,
			TxtRecords: []TXTRecord{
				{
					Value: []string{txtValue},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnszones/%s/TXT/%s?api-version=2018-05-01", subscriptionID, resourceGroup, zoneName, recordName)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnszones/%s/TXT/%s?api-version=2018-05-01", subscriptionID, resourceGroup, zoneName, recordName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 404 is expected if record doesn't exist
	if resp.StatusCode == 404 {
		return []string{}, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure API returned status %d: %s", resp.StatusCode, string(body))
	}

	var respData RecordSetResp
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var values []string
	for _, txtRec := range respData.Properties.TxtRecords {
		values = append(values, txtRec.Value...)
	}

	return values, nil
}

func (c *Client) DeleteTXTRecord(ctx context.Context, subscriptionID, resourceGroup, zoneName, recordName, value string) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}

	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/dnszones/%s/TXT/%s?api-version=2018-05-01", subscriptionID, resourceGroup, zoneName, recordName)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := http.DefaultClient.Do(req)
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
