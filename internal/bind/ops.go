package bind

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func Deploy(ctx context.Context, logger *slog.Logger, cfg Config) error {
	msg := newUpdateMessage(cfg, true)
	resp, err := exchange(ctx, msg, cfg)
	if err != nil {
		return fmt.Errorf("bind deploy exchange: %w", err)
	}
	if resp.Rcode != dns.RcodeSuccess {
		return fmt.Errorf("bind deploy failed with rcode %s", dns.RcodeToString[resp.Rcode])
	}

	logger.Info("deployed dns-01 TXT record", "fqdn", cfg.FQDN, "zone", cfg.DNSZone, "server", cfg.DNSServer)
	return nil
}

func Cleanup(ctx context.Context, logger *slog.Logger, cfg Config) error {
	msg := newUpdateMessage(cfg, false)
	resp, err := exchange(ctx, msg, cfg)
	if err != nil {
		logger.Warn("bind cleanup exchange failed; returning success for idempotency", "error", err.Error(), "fqdn", cfg.FQDN)
		return nil
	}

	if !cleanupRcodeOK(resp.Rcode) {
		logger.Warn("bind cleanup returned non-success; returning success for idempotency", "rcode", dns.RcodeToString[resp.Rcode], "fqdn", cfg.FQDN)
		return nil
	}

	logger.Info("cleanup completed", "fqdn", cfg.FQDN, "zone", cfg.DNSZone, "server", cfg.DNSServer)
	return nil
}

func newUpdateMessage(cfg Config, deploy bool) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetUpdate(dns.Fqdn(cfg.DNSZone))

	txt := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(cfg.FQDN),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    cfg.TTL,
		},
		Txt: []string{cfg.Validation},
	}

	if deploy {
		msg.Insert([]dns.RR{txt})
	} else {
		msg.Remove([]dns.RR{txt})
	}

	if cfg.TSIGKeyName != "" {
		msg.SetTsig(dns.Fqdn(cfg.TSIGKeyName), tsigAlgorithm(cfg.TSIGAlgorithm), 300, time.Now().Unix())
	}

	return msg
}

func exchange(ctx context.Context, msg *dns.Msg, cfg Config) (*dns.Msg, error) {
	client := &dns.Client{
		Net:     protocolForAddress(cfg.DNSServer),
		Timeout: 10 * time.Second,
	}

	if cfg.TSIGKeyName != "" {
		client.TsigSecret = map[string]string{dns.Fqdn(cfg.TSIGKeyName): cfg.TSIGSecret}
	}

	resp, _, err := client.ExchangeContext(ctx, msg, cfg.DNSServer)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("empty DNS response")
	}

	return resp, nil
}

func protocolForAddress(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "udp"
	}
	if strings.Contains(host, ":") {
		return "udp"
	}
	return "udp"
}

func cleanupRcodeOK(rcode int) bool {
	return rcode == dns.RcodeSuccess || rcode == dns.RcodeNameError || rcode == dns.RcodeNXRrset
}

func tsigAlgorithm(v string) string {
	a := strings.TrimSpace(strings.ToLower(v))
	if a == "" {
		return dns.HmacSHA256
	}
	if !strings.HasSuffix(a, ".") {
		a += "."
	}
	return a
}
