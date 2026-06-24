package main

import (
	"context"
	"os"
	"time"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/bind"
	"github.com/danieldonoghue/acme-gateway-hooks/internal/logging"
)

func main() {
	logs := logging.New("bind-dns-deploy")

	cfg, err := bind.LoadConfig()
	if err != nil {
		logs.Error.Error("invalid environment", "error", err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := bind.Deploy(ctx, logs.Info, cfg); err != nil {
		logs.Error.Error("deploy failed", "error", err.Error(), "zone", cfg.DNSZone, "fqdn", cfg.FQDN)
		os.Exit(1)
	}

	logs.Info.Info("deploy completed", "zone", cfg.DNSZone, "fqdn", cfg.FQDN)
}
