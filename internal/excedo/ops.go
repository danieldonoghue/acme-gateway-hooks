package excedo

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

func Deploy(ctx context.Context, logger *slog.Logger, client *Client, cfg Config) error {
	session, err := client.Login(ctx)
	if err != nil {
		return err
	}

	zone, recName, err := resolveZoneAndRecord(ctx, client, session, cfg.FQDN, cfg.DNSZone)
	if err != nil {
		return err
	}

	resp, err := client.AddTXTRecord(ctx, session, zone, recName, cfg.Validation, 120)
	if err != nil {
		return err
	}
	if !isSuccessCode(resp.Code) {
		return fmt.Errorf("add TXT record failed with API code %d", resp.Code)
	}

	logger.Info("deployed dns-01 TXT record", "fqdn", cfg.FQDN, "zone", zone, "record_name", recName)
	return nil
}

func Cleanup(ctx context.Context, logger *slog.Logger, client *Client, cfg Config) error {
	session, err := client.Login(ctx)
	if err != nil {
		logger.Warn("cleanup login failed; returning success for idempotency", "error", err.Error())
		return nil
	}

	zone, recName, err := resolveZoneAndRecord(ctx, client, session, cfg.FQDN, cfg.DNSZone)
	if err != nil {
		logger.Warn("cleanup could not resolve zone; returning success for idempotency", "fqdn", cfg.FQDN, "error", err.Error())
		return nil
	}

	recordsResp, err := client.GetRecords(ctx, session, zone)
	if err != nil {
		logger.Warn("cleanup could not fetch records; returning success for idempotency", "zone", zone, "error", err.Error())
		return nil
	}
	if !isSuccessCode(recordsResp.Code) {
		logger.Warn("cleanup getrecords non-success; returning success for idempotency", "zone", zone, "api_code", recordsResp.Code)
		return nil
	}

	matched := findMatchingRecords(recordsResp, cfg.FQDN, recName, cfg.Validation)
	if len(matched) == 0 {
		logger.Info("no matching TXT records found; cleanup is idempotent", "fqdn", cfg.FQDN)
		return nil
	}

	for _, recordID := range matched {
		delResp, delErr := client.DeleteRecord(ctx, session, zone, recordID)
		if delErr != nil {
			logger.Warn("delete record request failed; continuing", "record_id", recordID, "error", delErr.Error())
			continue
		}
		if !isSuccessCode(delResp.Code) && delResp.Code != NotFoundCode {
			logger.Warn("delete record API returned non-success; continuing", "record_id", recordID, "api_code", delResp.Code)
			continue
		}
	}

	logger.Info("cleanup completed", "fqdn", cfg.FQDN, "deleted_candidates", len(matched))
	return nil
}

func resolveZoneAndRecord(ctx context.Context, client *Client, sessionToken, fqdn, configuredZone string) (string, string, error) {
	if configuredZone != "" {
		return configuredZone, RelativeRecordName(fqdn, configuredZone), nil
	}

	candidates := ZoneCandidates(fqdn)
	for _, zone := range candidates {
		resp, err := client.GetRecords(ctx, sessionToken, zone)
		if err != nil {
			continue
		}
		if isSuccessCode(resp.Code) {
			return zone, RelativeRecordName(fqdn, zone), nil
		}
	}
	return "", "", fmt.Errorf("could not resolve DNS zone for %q", fqdn)
}

func ZoneCandidates(fqdn string) []string {
	fqdn = strings.Trim(strings.ToLower(strings.TrimSpace(fqdn)), ".")
	labels := strings.Split(fqdn, ".")
	if len(labels) < 2 {
		return []string{fqdn}
	}

	candidates := make([]string, 0, len(labels)-1)
	for i := 1; i < len(labels)-1; i++ {
		candidates = append(candidates, strings.Join(labels[i:], "."))
	}
	candidates = append(candidates, strings.Join(labels[len(labels)-2:], "."))
	return uniq(candidates)
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

func findMatchingRecords(resp *GetRecordsResponse, fqdn, relativeName, expectedValue string) []string {
	fqdn = strings.Trim(strings.ToLower(fqdn), ".")
	relativeName = strings.Trim(strings.ToLower(relativeName), ".")
	expectedValue = normalizeTXTValue(expectedValue)

	ids := make([]string, 0)
	seen := map[string]struct{}{}
	for _, block := range resp.DNS {
		for _, record := range block.Records {
			if strings.ToUpper(record.Type) != "TXT" {
				continue
			}
			name := strings.Trim(strings.ToLower(record.Name), ".")
			if name != fqdn && name != relativeName {
				continue
			}
			if normalizeTXTValue(record.Content) != expectedValue {
				continue
			}
			if record.RecordID == "" {
				continue
			}
			if _, ok := seen[record.RecordID]; ok {
				continue
			}
			seen[record.RecordID] = struct{}{}
			ids = append(ids, record.RecordID)
		}
	}
	return ids
}

func normalizeTXTValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"`)
	return v
}

func uniq(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
