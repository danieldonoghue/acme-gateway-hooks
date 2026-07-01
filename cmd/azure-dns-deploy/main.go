package main

import (
	"context"
	"os"
	"time"

	"github.com/danieldonoghue/acme-gateway-hooks/internal/azure"
	"github.com/danieldonoghue/acme-gateway-hooks/internal/logging"
)

func main() {
	logs := logging.New("azure-dns-deploy")

	cfg, err := azure.LoadConfig()
	if err != nil {
		logs.Error.Error("invalid environment", "error", err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := azure.NewClient(ctx, cfg)
	if err != nil {
		logs.Error.Error("failed to create Azure client", "error", err.Error())
		os.Exit(1)
	}

	if err := azure.Deploy(ctx, logs.Info, client, cfg); err != nil {
		logs.Error.Error("deploy failed", "error", err.Error(), "domain", cfg.Domain, "fqdn", cfg.FQDN)
		os.Exit(1)
	}

	logs.Info.Info("deploy completed", "domain", cfg.Domain, "fqdn", cfg.FQDN)
}
