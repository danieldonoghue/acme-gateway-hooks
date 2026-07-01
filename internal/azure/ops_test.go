package azure

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
)

func TestDeploy(t *testing.T) {
	recordsCreated := make(map[string]string)

	mockClient := &MockClient{
		createFn: func(ctx context.Context, sub, rg, zone, name, value string, ttl int32) (*RecordSetResponse, error) {
			recordsCreated[name] = value
			return &RecordSetResponse{
				ID:    "id123",
				Name:  name,
				Value: value,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{
		CommonConfig: env.CommonConfig{
			Domain:     "app.example.com",
			Validation: "test-value",
			FQDN:       "_acme-challenge.app.example.com",
		},
		SubscriptionID: "sub",
		ResourceGroup:  "rg",
		ZoneName:       "example.com",
	}

	err := deployWithMock(context.Background(), logger, mockClient, cfg)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if len(recordsCreated) != 1 {
		t.Errorf("expected 1 record created, got %d", len(recordsCreated))
	}

	expectedName := "_acme-challenge.app"
	if _, ok := recordsCreated[expectedName]; !ok {
		t.Errorf("expected record '%s' to be created", expectedName)
	}

	if recordsCreated[expectedName] != "test-value" {
		t.Errorf("expected value 'test-value', got '%s'", recordsCreated[expectedName])
	}
}

func TestCleanup(t *testing.T) {
	recordsListed := []string{"test-value"}
	recordsDeleted := make(map[string]struct{})

	mockClient := &MockClient{
		listFn: func(ctx context.Context, sub, rg, zone, name string) ([]string, error) {
			return recordsListed, nil
		},
		deleteFn: func(ctx context.Context, sub, rg, zone, name, value string) error {
			recordsDeleted[name] = struct{}{}
			return nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{
		CommonConfig: env.CommonConfig{
			Domain:     "app.example.com",
			Validation: "test-value",
			FQDN:       "_acme-challenge.app.example.com",
		},
		SubscriptionID: "sub",
		ResourceGroup:  "rg",
		ZoneName:       "example.com",
	}

	err := cleanupWithMock(context.Background(), logger, mockClient, cfg)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	expectedName := "_acme-challenge.app"
	if _, ok := recordsDeleted[expectedName]; !ok {
		t.Errorf("expected record '%s' to be deleted", expectedName)
	}
}

func TestCleanupNoRecords(t *testing.T) {
	mockClient := &MockClient{
		listFn: func(ctx context.Context, sub, rg, zone, name string) ([]string, error) {
			return []string{}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{
		CommonConfig: env.CommonConfig{
			Domain:     "app.example.com",
			Validation: "test-value",
			FQDN:       "_acme-challenge.app.example.com",
		},
		SubscriptionID: "sub",
		ResourceGroup:  "rg",
		ZoneName:       "example.com",
	}

	// Should not error when no records found
	err := cleanupWithMock(context.Background(), logger, mockClient, cfg)
	if err != nil {
		t.Fatalf("Cleanup should succeed with no records, got error: %v", err)
	}
}

func TestFindMatchingRecords(t *testing.T) {
	tests := []struct {
		name     string
		records  []string
		value    string
		expected int
	}{
		{
			name:     "exact match",
			records:  []string{"challenge-value"},
			value:    "challenge-value",
			expected: 1,
		},
		{
			name:     "quoted match",
			records:  []string{`"challenge-value"`},
			value:    "challenge-value",
			expected: 1,
		},
		{
			name:     "whitespace normalization",
			records:  []string{" challenge-value "},
			value:    "challenge-value",
			expected: 1,
		},
		{
			name:     "no match",
			records:  []string{"other-value"},
			value:    "challenge-value",
			expected: 0,
		},
		{
			name:     "multiple records",
			records:  []string{"value1", "challenge-value", "value2"},
			value:    "challenge-value",
			expected: 1,
		},
		{
			name:     "empty list",
			records:  []string{},
			value:    "challenge-value",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := findMatchingRecords(tt.records, tt.value)
			if len(matched) != tt.expected {
				t.Errorf("expected %d matches, got %d: %v", tt.expected, len(matched), matched)
			}
		})
	}
}

func TestNormalizeTXTValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "value",
			expected: "value",
		},
		{
			input:    `"value"`,
			expected: "value",
		},
		{
			input:    "  value  ",
			expected: "value",
		},
		{
			input:    `  "value"  `,
			expected: "value",
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeTXTValue(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeTXTValue(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Mock client for testing
type MockClient struct {
	createFn func(context.Context, string, string, string, string, string, int32) (*RecordSetResponse, error)
	listFn   func(context.Context, string, string, string, string) ([]string, error)
	deleteFn func(context.Context, string, string, string, string, string) error
}

func (m *MockClient) CreateTXTRecord(ctx context.Context, sub, rg, zone, name, value string, ttl int32) (*RecordSetResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, sub, rg, zone, name, value, ttl)
	}
	return nil, nil
}

func (m *MockClient) ListTXTRecords(ctx context.Context, sub, rg, zone, name string) ([]string, error) {
	if m.listFn != nil {
		return m.listFn(ctx, sub, rg, zone, name)
	}
	return []string{}, nil
}

func (m *MockClient) DeleteTXTRecord(ctx context.Context, sub, rg, zone, name, value string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, sub, rg, zone, name, value)
	}
	return nil
}

// Wrapper functions that use mock client
func deployWithMock(ctx context.Context, logger *slog.Logger, client *MockClient, cfg Config) error {
	recordName := RelativeRecordName(cfg.FQDN, cfg.ZoneName)

	resp, err := client.CreateTXTRecord(ctx, cfg.SubscriptionID, cfg.ResourceGroup, cfg.ZoneName, recordName, cfg.Validation, 120)
	if err != nil {
		return err
	}

	logger.Info("deployed dns-01 TXT record",
		"fqdn", cfg.FQDN,
		"zone", cfg.ZoneName,
		"record_name", recordName,
		"record_id", resp.ID,
	)
	return nil
}

func cleanupWithMock(ctx context.Context, logger *slog.Logger, client *MockClient, cfg Config) error {
	recordName := RelativeRecordName(cfg.FQDN, cfg.ZoneName)

	records, err := client.ListTXTRecords(ctx, cfg.SubscriptionID, cfg.ResourceGroup, cfg.ZoneName, recordName)
	if err != nil {
		logger.Warn("cleanup could not list records; returning success for idempotency",
			"zone", cfg.ZoneName,
			"record_name", recordName,
			"error", err.Error(),
		)
		return nil
	}

	matched := findMatchingRecords(records, cfg.Validation)
	if len(matched) == 0 {
		logger.Info("no matching TXT records found; cleanup is idempotent",
			"fqdn", cfg.FQDN,
			"zone", cfg.ZoneName,
		)
		return nil
	}

	for _, recordID := range matched {
		if err := client.DeleteTXTRecord(ctx, cfg.SubscriptionID, cfg.ResourceGroup, cfg.ZoneName, recordName, recordID); err != nil {
			logger.Warn("delete record request failed; continuing",
				"record_id", recordID,
				"error", err.Error(),
			)
			continue
		}
	}

	logger.Info("cleanup completed",
		"fqdn", cfg.FQDN,
		"deleted_candidates", len(matched),
	)
	return nil
}
