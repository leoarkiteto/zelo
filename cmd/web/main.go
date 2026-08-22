package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leoarkiteto/zelo/internal/auth"
	"github.com/leoarkiteto/zelo/internal/config"
	"github.com/leoarkiteto/zelo/internal/handler"
	"github.com/leoarkiteto/zelo/internal/service"
	"github.com/leoarkiteto/zelo/internal/store"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "apply migrations and exit")
	seed := flag.Bool("seed", false, "bootstrap the first condominium and syndic (development only)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := store.Migrate(ctx, db, "migrations"); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}
	if *migrateOnly {
		logger.Info("migrations applied")
		return
	}
	if *seed {
		if err := seedFirstCondominium(ctx, logger, db, cfg.PasswordPepper); err != nil {
			logger.Error("seed failed", "error", err)
			os.Exit(1)
		}
		logger.Info("development seed complete")
		return
	}

	users := store.NewUserStore(db)
	roles := store.NewRoleStore(db)
	units := store.NewUnitStore(db)
	invitations := store.NewInvitationStore(db)
	sessions := store.NewSessionStore(db)
	audit := store.NewAuditStore(db)

	hasher := auth.NewPasswordHasher(cfg.PasswordPepper)
	tokens := handler.TokenHasher{}
	sessMgr := auth.NewSessionManager(sessions, cfg.IsProduction())

	deps := handler.Dependencies{
		Logger:      logger,
		Sessions:    sessMgr,
		Passwords:   hasher,
		Tokens:      tokens,
		Users:       users,
		Roles:       roles,
		Units:       units,
		Invitations: invitations,
		Audit:       audit,
		Registration: &service.RegistrationService{
			Users: users, Roles: roles, Invitations: invitations,
			Passwords: hasher, Tokens: tokens, Now: time.Now,
		},
		AuthService: &service.AuthService{
			Users: users, Roles: roles, Passwords: hasher, Audit: audit, Now: time.Now,
		},
		PasswordReset: &service.PasswordResetService{
			Users: users, Passwords: hasher, Tokens: tokens, Now: time.Now,
		},
		RoleService: &service.RoleService{Roles: roles, Audit: audit},
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler.NewRouter(deps),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("server listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("server stopped")
}
