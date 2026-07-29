package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"f1/internal/config"
	"f1/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server init: %v", err)
	}

	// Graceful shutdown: по SIGINT/SIGTERM отменяем контекст,
	// server.Run гасит HTTP-сервер и закрывает соединения с БД/Redis.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("web server listening on :%s", cfg.HTTPPort)
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
	log.Println("web server stopped gracefully")
}
