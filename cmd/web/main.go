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

	authhandlers "github.com/leoarkiteto/zelo/internal/features/auth/handlers"
	authservices "github.com/leoarkiteto/zelo/internal/features/auth/services"
	directoryhandlers "github.com/leoarkiteto/zelo/internal/features/directory/handlers"
	"github.com/leoarkiteto/zelo/internal/features/directory/repositories"
	dirservices "github.com/leoarkiteto/zelo/internal/features/directory/services"
	financehandlers "github.com/leoarkiteto/zelo/internal/features/finance/handlers"
	financerepositories "github.com/leoarkiteto/zelo/internal/features/finance/repositories"
	financeservices "github.com/leoarkiteto/zelo/internal/features/finance/services"
	homehandlers "github.com/leoarkiteto/zelo/internal/features/home/handlers"
	managementhandlers "github.com/leoarkiteto/zelo/internal/features/management/handlers"
	mgmtservices "github.com/leoarkiteto/zelo/internal/features/management/services"
	profilehandlers "github.com/leoarkiteto/zelo/internal/features/profile/handlers"
	profileservices "github.com/leoarkiteto/zelo/internal/features/profile/services"
	tickethandlers "github.com/leoarkiteto/zelo/internal/features/tickets/handlers"
	ticketrepositories "github.com/leoarkiteto/zelo/internal/features/tickets/repositories"
	ticketservices "github.com/leoarkiteto/zelo/internal/features/tickets/services"
	"github.com/leoarkiteto/zelo/internal/shared/config"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "apply migrations and exit")
	seed := flag.Bool("seed", false, "populate the database with realistic development seed data (idempotent, development only)")
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
		if cfg.IsProduction() {
			logger.Error("seed refused: -seed is a development-only flag (APP_ENV=production)")
			os.Exit(1)
		}
		if err := seedDemoData(ctx, logger, db, cfg.PasswordPepper); err != nil {
			logger.Error("seed failed", "error", err)
			os.Exit(1)
		}
		return
	}

	redisClient, err := store.OpenRedis(cfg.RedisURL)
	if err != nil {
		logger.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	users := store.NewUserStore(db)
	roles := store.NewRoleStore(db)
	units := store.NewUnitStore(db)
	invitations := store.NewInvitationStore(db)
	sessions := store.NewRedisSessionStore(redisClient)
	audit := store.NewAuditStore(db)
	listings := repositories.NewListingStore(db)
	categories := repositories.NewCategoryStore(db)
	financeAccounts := financerepositories.NewAccountStore(db)
	ticketStore := ticketrepositories.NewTicketStore(db)

	hasher := security.NewPasswordHasher(cfg.PasswordPepper)
	tokens := security.TokenHasher{}
	sessMgr := security.NewSessionManager(sessions, cfg.IsProduction())

	authDeps := authhandlers.Deps{
		Logger:      logger,
		Sessions:    sessMgr,
		Invitations: invitations,
		Tokens:      tokens,
		Audit:       audit,
		Registration: &authservices.RegistrationService{
			CreateUser:               users.CreateUser,
			GrantRole:                roles.GrantRole,
			GetInvitationByTokenHash: invitations.GetInvitationByTokenHash,
			MarkInvitationAccepted:   invitations.MarkInvitationAccepted,
			HashPassword:             hasher.Hash,
			ValidatePassword:         hasher.ValidatePassword,
			HashToken:                tokens.HashToken,
			Now:                      time.Now,
		},
		AuthService: &authservices.AuthService{
			GetUserByEmail:     users.GetUserByEmail,
			RecordFailedSignIn: users.RecordFailedSignIn,
			ApplyLock:          users.ApplyLock,
			ClearLock:          users.ClearLock,
			FirstActiveRole:    roles.FirstActiveRoleForUser,
			VerifyPassword:     hasher.Verify,
			RecordEvent:        audit.RecordEvent,
			Now:                time.Now,
		},
		PasswordReset: &authservices.PasswordResetService{
			GetUserByEmail:   users.GetUserByEmail,
			UpdatePassword:   users.UpdatePassword,
			HashPassword:     hasher.Hash,
			ValidatePassword: hasher.ValidatePassword,
			HashToken:        tokens.HashToken,
			Now:              time.Now,
		},
	}
	directoryDeps := directoryhandlers.Deps{
		Roles:      roles,
		Audit:      audit,
		Listings:   listings,
		Categories: categories,
		Directory: &dirservices.DirectoryService{
			Listings: listings, Categories: categories, Units: units, Audit: audit,
		},
	}
	managementDeps := managementhandlers.Deps{
		Roles:       roles,
		Audit:       audit,
		Tokens:      tokens,
		Invitations: invitations,
		Units:       units,
		RoleService: &mgmtservices.RoleService{Roles: roles, Audit: audit},
	}
	homeDeps := homehandlers.Deps{Roles: roles, Audit: audit}
	financeDeps := financehandlers.Deps{
		Roles:     roles,
		Audit:     audit,
		Accounts:  financeAccounts,
		Units:     units,
		UploadDir: cfg.UploadDir,
		Finance: &financeservices.FinanceService{
			Accounts:  financeAccounts,
			Summaries: financeAccounts,
			Units:     units,
			Audit:     audit,
			Now:       time.Now,
		},
	}
	profileDeps := profilehandlers.Deps{
		Logger: logger,
		Roles:  roles,
		Profile: &profileservices.ProfileService{
			Users:       users,
			Preferences: users,
		},
	}
	ticketsDeps := tickethandlers.Deps{
		Roles: roles,
		Audit: audit,
		Units: units,
		Tickets: &ticketservices.TicketService{
			Tickets: ticketStore,
			Units:   units,
			Audit:   audit,
		},
	}

	mux := http.NewServeMux()
	authhandlers.RegisterRoutes(mux, authDeps)
	homehandlers.RegisterRoutes(mux, homeDeps)
	directoryhandlers.RegisterRoutes(mux, directoryDeps)
	managementhandlers.RegisterRoutes(mux, managementDeps)
	profilehandlers.RegisterRoutes(mux, profileDeps)
	financehandlers.RegisterRoutes(mux, financeDeps)
	tickethandlers.RegisterRoutes(mux, ticketsDeps)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	var root http.Handler = mux
	root = middleware.Recover(logger)(root)
	root = middleware.Logging(logger)(root)
	root = middleware.SecurityHeaders(root)
	root = middleware.WithLocale(root)
	root = middleware.WithUser(sessMgr, users)(root)
	root = middleware.CSRF(sessMgr)(root)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           root,
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
