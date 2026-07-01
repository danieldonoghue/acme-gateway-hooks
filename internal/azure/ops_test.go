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

	err := Deploy(context.Background(), logger, mockClient, cfg)
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

	err := Cleanup(context.Background(), logger, mockClient, cfg)
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
	err := Cleanup(context.Background(), logger, mockClient, cfg)
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

// MockClient implements the Azure client interface for testing
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
