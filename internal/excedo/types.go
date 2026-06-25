package excedo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type AuthResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Desc    string `json:"desc"`
	// Some Excedo deployments include extra response metadata here.
	Parameters map[string]any `json:"parameters"`
	Token      string         `json:"token"`
}

func (r AuthResponse) ParametersToken() string {
	if r.Parameters == nil {
		return ""
	}
	raw, ok := r.Parameters["token"]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

type AddRecordResponse struct {
	Code       int            `json:"code"`
	Message    string         `json:"message"`
	Desc       string         `json:"desc"`
	Parameters map[string]any `json:"parameters"`
}

type DeleteRecordResponse struct {
	Code       int            `json:"code"`
	Message    string         `json:"message"`
	Desc       string         `json:"desc"`
	Parameters map[string]any `json:"parameters"`
}

type GetRecordsResponse struct {
	Code       int                    `json:"code"`
	Message    string                 `json:"message"`
	Desc       string                 `json:"desc"`
	Parameters map[string]any         `json:"parameters"`
	DNS        map[string]DomainBlock `json:"dns"`
}

func (r *GetRecordsResponse) UnmarshalJSON(data []byte) error {
	type alias struct {
		Code       int             `json:"code"`
		Message    string          `json:"message"`
		Desc       string          `json:"desc"`
		Parameters map[string]any  `json:"parameters"`
		DNSRaw     json.RawMessage `json:"dns"`
	}

	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	r.Code = a.Code
	r.Message = a.Message
	r.Desc = a.Desc
	r.Parameters = a.Parameters
	r.DNS = map[string]DomainBlock{}

	if len(a.DNSRaw) == 0 || string(a.DNSRaw) == "null" {
		return nil
	}

	if err := json.Unmarshal(a.DNSRaw, &r.DNS); err == nil {
		return nil
	}

	var dnsObj map[string]json.RawMessage
	if err := json.Unmarshal(a.DNSRaw, &dnsObj); err == nil {
		for k, raw := range dnsObj {
			if len(raw) == 0 || string(raw) == "null" {
				r.DNS[k] = DomainBlock{}
				continue
			}

			var block DomainBlock
			if err := json.Unmarshal(raw, &block); err == nil {
				r.DNS[k] = block
				continue
			}

			var recs []DNSRecord
			if err := json.Unmarshal(raw, &recs); err == nil {
				r.DNS[k] = DomainBlock{Records: recs}
				continue
			}

			// Keep best-effort behavior for unexpected domain payloads.
			r.DNS[k] = DomainBlock{}
		}
		return nil
	}

	var list []json.RawMessage
	if err := json.Unmarshal(a.DNSRaw, &list); err != nil {
		return fmt.Errorf("unsupported dns payload shape: %w", err)
	}

	for i, item := range list {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(item, &obj); err == nil {
			var block DomainBlock
			var key string
			if raw, ok := obj["domainname"]; ok {
				_ = json.Unmarshal(raw, &key)
			}
			if key == "" {
				if raw, ok := obj["name"]; ok {
					_ = json.Unmarshal(raw, &key)
				}
			}
			if key == "" {
				key = strconv.Itoa(i)
			}

			if raw, ok := obj["records"]; ok {
				_ = json.Unmarshal(raw, &block.Records)
			} else {
				_ = json.Unmarshal(item, &block)
			}
			r.DNS[key] = block
			continue
		}

		var block DomainBlock
		if err := json.Unmarshal(item, &block); err == nil {
			r.DNS[strconv.Itoa(i)] = block
		}
	}

	return nil
}

type DomainBlock struct {
	Records []DNSRecord `json:"records"`
}

func (b *DomainBlock) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		b.Records = nil
		return nil
	}

	if strings.HasPrefix(raw, "[") {
		var records []DNSRecord
		if err := json.Unmarshal(data, &records); err != nil {
			return err
		}
		b.Records = records
		return nil
	}

	type alias struct {
		Records json.RawMessage `json:"records"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	if len(a.Records) == 0 || string(a.Records) == "null" {
		b.Records = nil
		return nil
	}

	var records []DNSRecord
	if err := json.Unmarshal(a.Records, &records); err == nil {
		b.Records = records
		return nil
	}

	var byID map[string]DNSRecord
	if err := json.Unmarshal(a.Records, &byID); err == nil {
		keys := make([]string, 0, len(byID))
		for k := range byID {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		out := make([]DNSRecord, 0, len(byID))
		for _, k := range keys {
			record := byID[k]
			if strings.TrimSpace(record.RecordID) == "" {
				record.RecordID = k
			}
			out = append(out, record)
		}
		b.Records = out
		return nil
	}

	return fmt.Errorf("unsupported records payload shape")
}

type DNSRecord struct {
	RecordID string `json:"recordid"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
}

func (r *DNSRecord) UnmarshalJSON(data []byte) error {
	type alias struct {
		RecordID any    `json:"recordid"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Content  string `json:"content"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	switch v := a.RecordID.(type) {
	case string:
		r.RecordID = v
	case float64:
		r.RecordID = strconv.FormatInt(int64(v), 10)
	case json.Number:
		r.RecordID = v.String()
	case nil:
		r.RecordID = ""
	default:
		return fmt.Errorf("unsupported recordid type %T", v)
	}

	r.Name = a.Name
	r.Type = a.Type
	r.Content = a.Content
	return nil
}
