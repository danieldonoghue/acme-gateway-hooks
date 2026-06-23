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
	logs := logging.New("excedo-dns-deploy")

	cfg, err := env.Load()
	if err != nil {
		logs.Error.Error("invalid environment", "error", err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := excedo.NewClient(cfg.APIURL, cfg.APIToken)
	if err := excedo.Deploy(ctx, logs.Info, client, cfg); err != nil {
		logs.Error.Error("deploy failed", "error", err.Error(), "domain", cfg.Domain, "fqdn", cfg.FQDN)
		os.Exit(1)
	}

	logs.Info.Info("deploy completed", "domain", cfg.Domain, "fqdn", cfg.FQDN)
}
