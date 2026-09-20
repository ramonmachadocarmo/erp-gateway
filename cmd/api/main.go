package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erp/pkg/audit"
	"erp/pkg/config"
	"erp/pkg/httpserver"
	"erp/pkg/redisx"
	httpadapter "erp/services/gateway-service/internal/adapters/http"
	"erp/services/gateway-service/internal/application"
	"erp/services/gateway-service/internal/domain"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	routes := []domain.Route{
		{
			Name: "identity", Prefix: "/api/identity", Target: env("IDENTITY_BASE_URL", "http://localhost:8080"),
			Public:   []string{"/auth/login", "/auth/register"},
			AuthOnly: []string{"/auth/me", "/auth/logout"},
		},
		{
			Name: "config", Prefix: "/api/config", Target: env("CONFIG_BASE_URL", "http://localhost:8085"),
			// Sales registers customers (and their addresses) while taking orders,
			// purchasing does the same for suppliers.
			Grants: []domain.Grant{
				{PathPrefix: "/customers", Module: "sales"},
				{PathPrefix: "/suppliers", Module: "purchasing"},
				{PathPrefix: "/cep", Module: "sales"},
				{PathPrefix: "/cep", Module: "purchasing"},
			},
		},
		{Name: "stock", Prefix: "/api/stock", Target: env("STOCK_BASE_URL", "http://localhost:8081")},
		{Name: "sales", Prefix: "/api/sales", Target: env("SALES_BASE_URL", "http://localhost:8082")},
		{Name: "purchasing", Prefix: "/api/purchasing", Target: env("PURCHASING_BASE_URL", "http://localhost:8083")},
		{Name: "assets", Prefix: "/api/assets", Target: env("ASSETS_BASE_URL", "http://localhost:8087")},
		{Name: "cashflow", Prefix: "/api/cashflow", Target: env("CASHFLOW_BASE_URL", "http://localhost:8088")},
		{Name: "invoicing", Prefix: "/api/invoicing", Target: env("INVOICING_BASE_URL", "http://localhost:8084")},
		{Name: "bi", Prefix: "/api/bi", Target: env("BI_BASE_URL", "http://localhost:8089")},
		{Name: "reports", Prefix: "/api/reports", Target: env("REPORTS_BASE_URL", "http://localhost:8090")},
	}
	svc := application.New(routes)
	redisClient := redisx.Connect(cfg.Redis.Addr(), cfg.Redis.Password)
	defer redisClient.Close()
	auditLogger, err := audit.Connect(ctx, cfg.Mongo.URI(), env("MONGO_DB", "audit_log"))
	if err != nil {
		log.Fatalf("connect audit log: %v", err)
	}
	defer auditLogger.Close()
	engine := httpserver.New(cfg.ServiceName)
	httpadapter.New(svc, routes, cfg.JWTSecret, cfg.JWTIssuer, redisClient, auditLogger).Register(engine)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Printf("%s listening on %s", cfg.ServiceName, srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
