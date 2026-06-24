package integration

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/bind"
	"github.com/miekg/dns"
)

type fakeBindServer struct {
	mu      sync.Mutex
	records map[string]map[string]struct{}
}

func newFakeBindServer() *fakeBindServer {
	return &fakeBindServer{records: map[string]map[string]struct{}{}}
}

func (f *fakeBindServer) handleMessage(r *dns.Msg) *dns.Msg {
	resp := &dns.Msg{}
	resp.MsgHdr = dns.MsgHdr{
		Id:                 r.Id,
		Response:           true,
		Opcode:             r.Opcode,
		Authoritative:      true,
		RecursionAvailable: false,
		Rcode:              dns.RcodeSuccess,
	}

	if r.Opcode != dns.OpcodeUpdate {
		resp.SetRcode(r, dns.RcodeFormatError)
		return resp
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, rr := range r.Ns {
		txt, ok := rr.(*dns.TXT)
		if !ok || len(txt.Txt) == 0 {
			continue
		}

		name := dns.Fqdn(strings.ToLower(txt.Hdr.Name))
		value := txt.Txt[0]

		if txt.Hdr.Class == dns.ClassNONE {
			if values, exists := f.records[name]; exists {
				delete(values, value)
				if len(values) == 0 {
					delete(f.records, name)
				}
			}
			continue
		}

		if _, exists := f.records[name]; !exists {
			f.records[name] = map[string]struct{}{}
		}
		f.records[name][value] = struct{}{}
	}

	return resp
}

func (f *fakeBindServer) hasTXT(name, value string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	values, ok := f.records[dns.Fqdn(strings.ToLower(name))]
	if !ok {
		return false
	}
	_, ok = values[value]
	return ok
}

func TestBindDeployAndCleanupIdempotent(t *testing.T) {
	srvState := newFakeBindServer()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	defer pc.Close()

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		buf := make([]byte, 4096)
		for {
			n, addr, readErr := pc.ReadFrom(buf)
			if readErr != nil {
				return
			}

			req := new(dns.Msg)
			if unpackErr := req.Unpack(buf[:n]); unpackErr != nil {
				continue
			}

			resp := srvState.handleMessage(req)
			packed, packErr := resp.Pack()
			if packErr != nil {
				continue
			}

			_, _ = pc.WriteTo(packed, addr)
		}
	}()
	defer func() {
		_ = pc.Close()
		<-serveDone
	}()

	t.Setenv("CERTBOT_DOMAIN", "example.com")
	t.Setenv("CERTBOT_VALIDATION", "challenge-value")
	t.Setenv("ACME_GATEWAY_FQDN", "_acme-challenge.example.com")
	t.Setenv("BIND_DNS_SERVER", pc.LocalAddr().String())
	t.Setenv("BIND_DNS_ZONE", "example.com")
	t.Setenv("BIND_DNS_TTL", "60")

	cfg, err := bind.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ctx := context.Background()

	if err := bind.Deploy(ctx, logger, cfg); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !srvState.hasTXT(cfg.FQDN, cfg.Validation) {
		t.Fatalf("expected TXT record after deploy for %s", cfg.FQDN)
	}

	if err := bind.Cleanup(ctx, logger, cfg); err != nil {
		t.Fatalf("Cleanup first call: %v", err)
	}
	if srvState.hasTXT(cfg.FQDN, cfg.Validation) {
		t.Fatalf("expected TXT record removed after cleanup for %s", cfg.FQDN)
	}

	if err := bind.Cleanup(ctx, logger, cfg); err != nil {
		t.Fatalf("Cleanup second call should be idempotent: %v", err)
	}
}
