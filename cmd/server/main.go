package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"webapp/internal/config"
	"webapp/internal/handler"
	"webapp/internal/middleware"
	"webapp/internal/migrate"
	"webapp/internal/repository"
	"webapp/internal/service"
	"webapp/migrations"
	"webapp/web"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	db, err := repository.Connect(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run pending SQL migrations before accepting traffic.
	// Safe to call on every startup — already-applied files are skipped.
	if err := migrate.Run(context.Background(), db, migrations.FS); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}

	// Repositories
	userRepo := repository.NewUserRepo(db)
	deckRepo := repository.NewDeckRepo(db)
	cardRepo := repository.NewCardRepo(db)
	reviewRepo := repository.NewReviewRepo(db)
	studyRepo := repository.NewStudyRepo(db)
	uploadRepo := repository.NewUploadRepo(db)

	// Services
	healthSvc := service.NewHealthService(db)
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTExpiry, cfg.AdminEmails)
	studySvc := service.NewStudyService(studyRepo, cardRepo, reviewRepo)
	contentSvc := service.NewContentService(db, deckRepo, cardRepo, uploadRepo)
	adminSvc := service.NewAdminService(userRepo, logger)

	// OAuth
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	// Handlers
	healthH := handler.NewHealthHandler(healthSvc)
	authH := handler.NewAuthHandler(oauthCfg, authSvc, cfg.CookieSecure)
	meH := handler.NewMeHandler(userRepo)
	studyH := handler.NewStudyHandler(studySvc)
	contentH := handler.NewContentHandler(contentSvc)
	adminH := handler.NewAdminHandler(adminSvc)

	// --- Access-level helpers (RBAC) ---
	jwtSecret := []byte(cfg.JWTSecret)
	authOnly := middleware.RequireAuth(jwtSecret)
	contentMgmt := middleware.Chain(middleware.RequireAuth(jwtSecret), middleware.RequireRole("professor", "admin"))
	adminOnly := middleware.Chain(middleware.RequireAuth(jwtSecret), middleware.RequireRole("admin"))

	mux := http.NewServeMux()

	// Public: health
	mux.HandleFunc("GET /healthz", healthH.Healthz)
	mux.HandleFunc("GET /readyz", healthH.Readyz)

	// Public: OAuth flow
	mux.HandleFunc("GET /auth/google", authH.GoogleRedirect)
	mux.HandleFunc("GET /auth/google/callback", authH.GoogleCallback)
	mux.HandleFunc("POST /auth/logout", authH.Logout)

	// Authenticated: any role
	mux.Handle("GET /api/me", authOnly(http.HandlerFunc(meH.Me)))
	mux.Handle("GET /api/decks", authOnly(http.HandlerFunc(studyH.ListDecks)))
	mux.Handle("GET /api/study/next", authOnly(http.HandlerFunc(studyH.NextCard)))
	mux.Handle("GET /api/study/topics", authOnly(http.HandlerFunc(studyH.Topics)))
	mux.Handle("POST /api/study/answer", authOnly(http.HandlerFunc(studyH.SubmitAnswer)))
	mux.Handle("GET /api/stats", authOnly(http.HandlerFunc(studyH.Stats)))
	mux.Handle("GET /api/progress", authOnly(http.HandlerFunc(studyH.Progress)))

	// Content management: professor or admin
	mux.Handle("GET /api/content/decks", contentMgmt(http.HandlerFunc(studyH.ListDecksForManagement)))
	mux.Handle("GET /api/content/cards", contentMgmt(http.HandlerFunc(contentH.ListCards)))
	mux.Handle("GET /api/content/cards/{id}", contentMgmt(http.HandlerFunc(contentH.GetCardDetail)))
	mux.Handle("POST /api/content/decks", contentMgmt(http.HandlerFunc(contentH.CreateDeck)))
	mux.Handle("GET /api/content/decks/{id}", contentMgmt(http.HandlerFunc(contentH.GetDeck)))
	mux.Handle("PUT /api/content/decks/{id}", contentMgmt(http.HandlerFunc(contentH.UpdateDeck)))
	mux.Handle("PATCH /api/content/decks/{id}", contentMgmt(http.HandlerFunc(contentH.PatchDeck)))
	mux.Handle("DELETE /api/content/decks/{id}", contentMgmt(http.HandlerFunc(contentH.DeleteDeck)))
	mux.Handle("POST /api/content/cards", contentMgmt(http.HandlerFunc(contentH.CreateCard)))
	mux.Handle("PUT /api/content/cards/{id}", contentMgmt(http.HandlerFunc(contentH.UpdateCard)))
	mux.Handle("DELETE /api/content/cards/{id}", contentMgmt(http.HandlerFunc(contentH.DeleteCard)))
	mux.Handle("POST /api/content/upload-csv", contentMgmt(http.HandlerFunc(contentH.UploadCSV)))
	mux.Handle("GET /api/content/decks/{id}/export.csv", contentMgmt(http.HandlerFunc(contentH.ExportDeckCSV)))

	// Professor / admin stats (no individual student data — aggregates only)
	mux.Handle("GET /api/admin/stats", contentMgmt(http.HandlerFunc(studyH.ProfessorStats)))

	// Admin only
	mux.Handle("GET /api/admin/users", adminOnly(http.HandlerFunc(adminH.ListUsers)))
	mux.Handle("POST /api/admin/users/{id}/roles", adminOnly(http.HandlerFunc(adminH.SetRoles)))

	// Static assets + HTML pages (wildcard catch-all via embed.go *.html)
	staticFS, err := fs.Sub(web.Content, "static")
	if err != nil {
		logger.Error("failed to load embedded assets", "error", err)
		os.Exit(1)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
	// Service worker must be served from root scope
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, err := web.Content.ReadFile("static/sw.js")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("GET /{$}", serveHTML("index.html"))
	mux.HandleFunc("GET /study.html", serveHTML("study.html"))
	mux.HandleFunc("GET /me.html", serveHTML("me.html"))
	mux.HandleFunc("GET /progress.html", serveHTML("progress.html"))
	mux.HandleFunc("GET /teach.html", serveHTML("teach.html"))
	mux.HandleFunc("GET /deck_manage.html", serveHTML("deck_manage.html"))
	mux.HandleFunc("GET /admin_users.html", serveHTML("admin_users.html"))
	mux.HandleFunc("GET /professor_stats.html", serveHTML("professor_stats.html"))

	// Global middleware stack
	isDev := cfg.Environment != "production"
	rateLimiter := middleware.NewTieredRateLimiterStore([]middleware.TieredConfig{
		{Prefix: "/auth/", RPS: cfg.AuthRateLimitRPS, Burst: cfg.AuthRateLimitBurst},
		{Prefix: "/api/", RPS: cfg.APIRateLimitRPS, Burst: cfg.APIRateLimitBurst},
	})
	stack := middleware.Chain(
		middleware.RequestID,
		middleware.Logger(logger),
		middleware.SecurityHeaders(!isDev),
		middleware.CORS(cfg.AllowedOrigins),
		middleware.CSRF(cfg.AllowedOrigins, isDev),
		middleware.MaxBody(cfg.MaxBodySize),
		middleware.RateLimit(rateLimiter),
	)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      stack(mux),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server starting", "port", cfg.Port, "env", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced shutdown", "error", err)
	}
}

func serveHTML(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, err := web.Content.ReadFile(name)
		if err != nil {
			handler.Error(w, http.StatusInternalServerError, "page not found")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}
