package main

import (
	"context"
	"os"
	"time"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/env"
	"github.com/danieldonoghue/acme-gateway-hooks/internal/excedo"
	"github.com/danieldonoghue/acme-gateway-hooks/internal/logging"
)

func main() {
	logs := logging.New("excedo-dns-cleanup")

	cfg, err := env.Load()
	if err != nil {
		logs.Error.Error("invalid environment", "error", err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := excedo.NewClient(cfg.APIURL, cfg.APIToken)
	if err := excedo.Cleanup(ctx, logs.Info, client, cfg); err != nil {
		// Keep cleanup best-effort and idempotent by returning success after logging.
		logs.Error.Error("cleanup encountered error; returning success for idempotency", "error", err.Error(), "domain", cfg.Domain, "fqdn", cfg.FQDN)
		return
	}

	logs.Info.Info("cleanup completed", "domain", cfg.Domain, "fqdn", cfg.FQDN)
}
