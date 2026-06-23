package excedo

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type AuthResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Token   string `json:"token"`
}

type AddRecordResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type DeleteRecordResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type GetRecordsResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	DNS     map[string]DomainBlock `json:"dns"`
}

type DomainBlock struct {
	Records []DNSRecord `json:"records"`
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
