package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	fabricconfig "github.com/CHESSComputing/FabricNode/pkg/config"
	"github.com/CHESSComputing/FabricNode/pkg/server"
	"github.com/CHESSComputing/FabricNode/services/notification-service/internal/handlers"
	"github.com/CHESSComputing/FabricNode/services/notification-service/internal/store"
)

func main() {
	// ── Load & validate configuration ────────────────────────────────────────
	cfg, err := fabricconfig.Load(server.GetEnv("FABRIC_CONFIG", ""))
	if err != nil {
		log.Printf("notification-service: config warning: %v — using defaults", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("notification-service: %v", err)
	}

	hcfg := &handlers.Config{
		Inbox:  store.New(),
		NodeID: cfg.Node.ID,
	}

	// ── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(server.ReadWriteCORS())

	r.Get("/inbox", handlers.InboxList(hcfg))
	r.Post("/inbox", handlers.InboxReceive(hcfg))
	r.Get("/inbox/{id}", handlers.InboxGet(hcfg))
	r.Post("/inbox/{id}/ack", handlers.InboxAck(hcfg))
	r.Get("/inbox/stats", handlers.InboxStats(hcfg))
	r.Get("/health", handlers.Health(hcfg))
	r.Get("/", handlers.Index(hcfg))

	port := server.GetEnv("PORT", fmt.Sprintf("%d", cfg.Notification.Port))
	if cfg.TSLConfig.ServerKey == "" && cfg.TSLConfig.ServerCert == "" {
		log.Printf("HTTP notification-service listening on :%s (node: %s)", port, hcfg.NodeID)
		log.Fatal(http.ListenAndServe(":"+port, r))
	} else {
		log.Printf("HTTPs notification-service listening on :%s (node: %s)", port, hcfg.NodeID)
		log.Fatal(http.ListenAndServeTLS(":"+port, cfg.TSLConfig.ServerCert, cfg.TSLConfig.ServerKey, r))
	}
}
