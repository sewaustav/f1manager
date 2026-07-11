package server

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"time"

	authhandler "f1/internal/auth/handler"
	authrepo "f1/internal/auth/repo"
	authservice "f1/internal/auth/service"
	"f1/internal/config"
	"f1/internal/db"
	"f1/internal/engine"
	"f1/internal/new_storage/stub"
	"f1/internal/service"
	"f1/internal/web/connection"
	"f1/internal/web/dispatcher"
	webhttp "f1/internal/web/handler/http"
	jwtmw "f1/pkg/middleware/jwt"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

type Server struct {
	httpServer *http.Server
	database   *db.DataBase
}

// New собирает весь граф зависимостей приложения.
func New(cfg config.Config) (*Server, error) {
	database := &db.DataBase{}
	if err := database.Open(cfg.DB.Name, cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port); err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	privateKey, publicKey, err := loadKeys(cfg.JWT.PrivateKeyPath, cfg.JWT.PublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load jwt keys: %w", err)
	}

	// auth
	authSvc := authservice.New(
		authrepo.NewPostgres(database.GetDB()),
		privateKey,
		cfg.JWT.Issuer, cfg.JWT.Audience,
		cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL,
	)
	authHandler := authhandler.New(authSvc)
	middleware := jwtmw.New(publicKey, cfg.JWT.Issuer, cfg.JWT.Audience)

	// игровой граф
	manager := connection.NewManager()
	eng := engine.NewEngine(stub.NewEngineRepo())
	svc := service.New(stub.NewStatic(), stub.NewDynamic(), eng, service.NewMemoryUpdateCache(), manager)
	disp := dispatcher.New(svc, manager)
	gameHandler := webhttp.NewHttpHandler(svc, svc, svc, svc, manager, disp)

	draftDisp := dispatcher.NewDraft(svc, manager)
	draftHandler := webhttp.NewDraftHandler(draftDisp, svc)

	router := setupRouter(cfg, authHandler, draftHandler, gameHandler, middleware)

	return &Server{
		httpServer: &http.Server{
			Addr:    ":" + cfg.HTTPPort,
			Handler: router,
		},
		database: database,
	}, nil
}

// Run запускает HTTP-сервер и гасит его при отмене контекста.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(shutdownCtx)
	s.database.Close()
	return err
}

func loadKeys(privatePath, publicPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privPEM, err := os.ReadFile(privatePath)
	if err != nil {
		return nil, nil, err
	}
	priv, err := jwtlib.ParseRSAPrivateKeyFromPEM(privPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("parse private key: %w", err)
	}

	pubPEM, err := os.ReadFile(publicPath)
	if err != nil {
		return nil, nil, err
	}
	pub, err := jwtlib.ParseRSAPublicKeyFromPEM(pubPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("parse public key: %w", err)
	}

	return priv, pub, nil
}
