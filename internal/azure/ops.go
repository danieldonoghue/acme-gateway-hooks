package azure

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

type ClientInterface interface {
	CreateTXTRecord(ctx context.Context, sub, rg, zone, name, value string, ttl int32) (*RecordSetResponse, error)
	ListTXTRecords(ctx context.Context, sub, rg, zone, name string) ([]string, error)
	DeleteTXTRecord(ctx context.Context, sub, rg, zone, name, value string) error
}

func Deploy(ctx context.Context, logger *slog.Logger, client ClientInterface, cfg Config) error {
	recordName := RelativeRecordName(cfg.FQDN, cfg.ZoneName)

	resp, err := client.CreateTXTRecord(ctx, cfg.SubscriptionID, cfg.ResourceGroup, cfg.ZoneName, recordName, cfg.Validation, 120)
	if err != nil {
		return fmt.Errorf("create TXT record failed: %w", err)
	}

	logger.Info("deployed dns-01 TXT record",
		"fqdn", cfg.FQDN,
		"zone", cfg.ZoneName,
		"record_name", recordName,
		"record_id", resp.ID,
	)
	return nil
}

func Cleanup(ctx context.Context, logger *slog.Logger, client ClientInterface, cfg Config) error {
	recordName := RelativeRecordName(cfg.FQDN, cfg.ZoneName)

	// List records to find matching ones
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

func RelativeRecordName(fqdn, zone string) string {
	fqdn = strings.Trim(strings.ToLower(strings.TrimSpace(fqdn)), ".")
	zone = strings.Trim(strings.ToLower(strings.TrimSpace(zone)), ".")

	suffix := "." + zone
	if strings.HasSuffix(fqdn, suffix) {
		name := strings.TrimSuffix(fqdn, suffix)
		if name != "" {
			return name
		}
	}
	return fqdn
}

func findMatchingRecords(records []string, expectedValue string) []string {
	expectedValue = normalizeTXTValue(expectedValue)

	ids := make([]string, 0)
	seen := map[string]struct{}{}
	for _, record := range records {
		if normalizeTXTValue(record) != expectedValue {
			continue
		}
		if record == "" {
			continue
		}
		if _, ok := seen[record]; ok {
			continue
		}
		seen[record] = struct{}{}
		ids = append(ids, record)
	}
	return ids
}

func normalizeTXTValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"`)
	return v
}
