package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/excedo"
)

type fakeExcedo struct {
	mu      sync.Mutex
	records map[string][]map[string]string
	nextID  int
}

func newFakeExcedo() *fakeExcedo {
	return &fakeExcedo{
		records: map[string][]map[string]string{"example.com": {}},
		nextID:  1,
	}
}

func (f *fakeExcedo) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/authenticate/login/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"code": 1000, "message": "ok", "token": "session"})
	})

	mux.HandleFunc("/dns/getrecords/", func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domainname")
		f.mu.Lock()
		recs, ok := f.records[domain]
		f.mu.Unlock()
		if !ok {
			writeJSON(w, map[string]any{"code": 2004, "message": "zone not found", "dns": map[string]any{}})
			return
		}
		writeJSON(w, map[string]any{
			"code":    1000,
			"message": "ok",
			"dns": map[string]any{
				domain: map[string]any{"records": recs},
			},
		})
	})

	mux.HandleFunc("/dns/addrecord/", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		domain := r.Form.Get("domainname")
		name := r.Form.Get("name")
		content := r.Form.Get("content")

		f.mu.Lock()
		id := f.nextID
		f.nextID++
		f.records[domain] = append(f.records[domain], map[string]string{
			"recordid": strconv.Itoa(id),
			"name":     name,
			"type":     "TXT",
			"content":  content,
		})
		f.mu.Unlock()

		writeJSON(w, map[string]any{"code": 1000, "message": "ok"})
	})

	mux.HandleFunc("/dns/deleterecord/", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		domain := r.Form.Get("domainname")
		recordID := r.Form.Get("recordid")

		f.mu.Lock()
		defer f.mu.Unlock()
		recs := f.records[domain]
		idx := -1
		for i, rec := range recs {
			if rec["recordid"] == recordID {
				idx = i
				break
			}
		}
		if idx == -1 {
			writeJSON(w, map[string]any{"code": 2303, "message": "not found"})
			return
		}
		f.records[domain] = append(recs[:idx], recs[idx+1:]...)
		writeJSON(w, map[string]any{"code": 1000, "message": "ok"})
	})

	return mux
}

func TestDeployAndCleanupIdempotent(t *testing.T) {
	api := newFakeExcedo()
	srv := httptest.NewServer(api.handler())
	defer srv.Close()

	t.Setenv("EXCEDO_API_TOKEN", "token")
	t.Setenv("EXCEDO_API_URL", srv.URL)
	t.Setenv("CERTBOT_DOMAIN", "example.com")
	t.Setenv("CERTBOT_VALIDATION", "challenge-value")
	cfg, err := excedo.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client := excedo.NewClient(cfg.APIURL, cfg.APIToken)
	ctx := context.Background()

	if err := excedo.Deploy(ctx, logger, client, cfg); err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if err := excedo.Cleanup(ctx, logger, client, cfg); err != nil {
		t.Fatalf("Cleanup first call: %v", err)
	}
	if err := excedo.Cleanup(ctx, logger, client, cfg); err != nil {
		t.Fatalf("Cleanup second call should be idempotent: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
