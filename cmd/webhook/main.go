package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"errors"

	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mixigroup/mixi2-application-sdk-go/auth"
	"github.com/mixigroup/mixi2-application-sdk-go/event/webhook"
	application_apiv1 "github.com/mixigroup/mixi2-application-sdk-go/gen/go/social/mixi/application/service/application_api/v1"
	"github.com/son3941/project-dasei-v2/config"
	"github.com/son3941/project-dasei-v2/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	cfg := config.GetConfig()
	if err := handler.LoadExternalDictionary(); err != nil {
		log.Fatalf("failed to load external dictionary: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Decode public key
	if cfg.SignaturePublicKey == "" {
		log.Fatal("SIGNATURE_PUBLIC_KEY is required")
	}
	publicKeyBytes, err := base64.StdEncoding.DecodeString(cfg.SignaturePublicKey)
	if err != nil {
		log.Fatalf("failed to decode public key: %v", err)
	}
	publicKey := ed25519.PublicKey(publicKeyBytes)

	// Create authenticator
	authenticator, err := auth.NewAuthenticator(cfg.ClientID, cfg.ClientSecret, cfg.TokenURL)
	if err != nil {
		log.Fatalf("failed to create authenticator: %v", err)
	}

	// Create gRPC connection for API
	apiConn, err := grpc.NewClient(
		cfg.APIAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)
	if err != nil {
		log.Fatalf("failed to connect to api: %v", err)
	}
	defer apiConn.Close()

	// Create API client
	apiClient := application_apiv1.NewApplicationServiceClient(apiConn)

	// Create event handler
	eventHandler := handler.NewHandler(apiClient, authenticator)
	go func() {
		for {
			time.Sleep(30 * time.Second)

			ctx := context.Background()
			if err := eventHandler.PostMutter(ctx); err != nil {
				logger.Error("mutter failed", "err", err)
			}
		}
	}()
	go func() {
		for {
			time.Sleep(3 * time.Hour)

			handler.StartForcedDaseiActivity(30 * time.Minute)
		}
	}()
	// Create server
	addr := ":" + cfg.Port

	webhookServer := webhook.NewServer(
		addr,
		publicKey,
		eventHandler,
		webhook.WithLogger(logger),
	)

	mux := http.NewServeMux()

	// Health check for Render / UptimeRobot
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// All other requests go to the mixi2 webhook server
	mux.Handle("/", webhookServer.Handler())

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Error("shutdown error", slog.Any("error", err))
		}
	}()

	// Start server
	logger.Info("starting webhook server", slog.String("port", cfg.Port))
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}

	logger.Info("stopped")
}
