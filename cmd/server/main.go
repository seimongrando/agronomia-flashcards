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
	"webapp/internal/push"
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
	classRepo := repository.NewClassRepo(db)
	classStatsRepo := repository.NewClassStatsRepo(db)
	pushRepo := repository.NewPushRepo(db)

	// Services
	healthSvc := service.NewHealthService(db)
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTExpiry, cfg.AdminEmails)
	studySvc := service.NewStudyService(studyRepo, cardRepo, reviewRepo)
	contentSvc := service.NewContentService(db, deckRepo, cardRepo, uploadRepo)
	adminSvc := service.NewAdminService(userRepo, logger)
	classSvc := service.NewClassService(classRepo, classStatsRepo)

	pushClient, err := push.NewClient(cfg.VAPIDPrivateKey, cfg.VAPIDPublicKey, cfg.VAPIDSubject)
	if err != nil {
		logger.Error("failed to load VAPID keys", "error", err)
		os.Exit(1)
	}
	pushSvc := service.NewPushService(pushRepo, pushClient)

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
	classH := handler.NewClassHandler(classSvc)
	pushH := handler.NewPushHandler(pushSvc)

	// --- Access-level helpers (RBAC) ---
	jwtSecret := []byte(cfg.JWTSecret)
	authOnly := middleware.RequireAuth(jwtSecret)
	contentMgmt := middleware.Chain(middleware.RequireAuth(jwtSecret), middleware.RequireRole("professor", "admin"))
	adminOnly := middleware.Chain(middleware.RequireAuth(jwtSecret), middleware.RequireRole("admin"))
	staffOnly := middleware.Chain(middleware.RequireAuth(jwtSecret), middleware.RequireRole("professor", "admin"))

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
	mux.Handle("GET /api/study/offline", authOnly(http.HandlerFunc(studyH.OfflineBundle)))
	mux.Handle("POST /api/me/deck-hidden", authOnly(http.HandlerFunc(studyH.HideDeck)))

	// Push notifications (any authenticated user)
	mux.HandleFunc("GET /api/push/key", pushH.PublicKey)
	mux.Handle("POST /api/push/subscribe", authOnly(http.HandlerFunc(pushH.Subscribe)))
	mux.Handle("DELETE /api/push/subscribe", authOnly(http.HandlerFunc(pushH.Unsubscribe)))

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

	// Classes — authenticated (all roles can list/join; staff can create/manage).
	// Exact-path routes are listed before wildcard routes for clarity
	// (Go 1.22+ mux resolves by specificity regardless of order, but explicit is clearer).
	mux.Handle("GET /api/classes", authOnly(http.HandlerFunc(classH.ListMyClasses)))
	mux.Handle("POST /api/classes", staffOnly(http.HandlerFunc(classH.CreateClass)))
	mux.Handle("POST /api/classes/join", authOnly(http.HandlerFunc(classH.JoinClass)))
	mux.Handle("GET /api/classes/overview", staffOnly(http.HandlerFunc(classH.ClassOverview)))
	mux.Handle("GET /api/classes/{id}", staffOnly(http.HandlerFunc(classH.GetClass)))
	mux.Handle("PUT /api/classes/{id}", staffOnly(http.HandlerFunc(classH.UpdateClass)))
	mux.Handle("DELETE /api/classes/{id}", staffOnly(http.HandlerFunc(classH.DeleteClass)))
	mux.Handle("DELETE /api/classes/{id}/leave", authOnly(http.HandlerFunc(classH.LeaveClass)))
	mux.Handle("GET /api/classes/{id}/decks", authOnly(http.HandlerFunc(classH.ListClassDecks)))
	mux.Handle("POST /api/classes/{id}/invite", staffOnly(http.HandlerFunc(classH.RegenerateInviteCode)))
	mux.Handle("POST /api/classes/{id}/decks", staffOnly(http.HandlerFunc(classH.AssignDeck)))
	mux.Handle("DELETE /api/classes/{id}/decks/{deckId}", staffOnly(http.HandlerFunc(classH.UnassignDeck)))
	mux.Handle("GET /api/classes/{id}/stats", staffOnly(http.HandlerFunc(classH.ClassStats)))

	// Student private decks & cards
	mux.Handle("GET /api/my/decks", authOnly(http.HandlerFunc(contentH.ListMyDecks)))
	mux.Handle("POST /api/my/decks", authOnly(http.HandlerFunc(contentH.CreateMyDeck)))
	mux.Handle("DELETE /api/my/decks/{id}", authOnly(http.HandlerFunc(contentH.DeleteMyDeck)))
	mux.Handle("GET /api/my/decks/{id}/cards", authOnly(http.HandlerFunc(contentH.ListMyDeckCards)))
	mux.Handle("POST /api/my/decks/{id}/cards", authOnly(http.HandlerFunc(contentH.CreateMyCard)))
	mux.Handle("PUT /api/my/cards/{id}", authOnly(http.HandlerFunc(contentH.UpdateMyCard)))
	mux.Handle("DELETE /api/my/cards/{id}", authOnly(http.HandlerFunc(contentH.DeleteMyCard)))

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
	mux.HandleFunc("GET /classes.html", serveHTML("classes.html"))
	mux.HandleFunc("GET /class_manage.html", serveHTML("class_manage.html"))
	mux.HandleFunc("GET /my_decks.html", serveHTML("my_decks.html"))
	mux.HandleFunc("GET /my_deck.html", serveHTML("my_deck.html"))

	// Global middleware stack
	isDev := cfg.Environment != "production"
	rateLimiter := middleware.NewTieredRateLimiterStore([]middleware.TieredConfig{
		{Prefix: "/auth/", RPS: cfg.AuthRateLimitRPS, Burst: cfg.AuthRateLimitBurst},
		{Prefix: "/api/", RPS: cfg.APIRateLimitRPS, Burst: cfg.APIRateLimitBurst},
	})
	rateLimiter.SetTrustedProxy(cfg.TrustedProxy)
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

	// Daily push notification scheduler.
	// Fires once per day at cfg.PushNotifyHour UTC, reminding users with due cards.
	go runDailyScheduler(ctx, logger, pushSvc, cfg.PushNotifyHour)

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

// runDailyScheduler fires SendDueReminders once per day at the configured UTC hour.
// It wakes up every hour and checks if the current hour matches; this ensures the
// job fires even if the server restarts within the target hour window.
func runDailyScheduler(ctx context.Context, log *slog.Logger, svc *service.PushService, targetHour int) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	lastFired := -1 // track last day of year we fired (prevents double-fire within same hour)
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			if t.UTC().Hour() == targetHour && t.UTC().YearDay() != lastFired {
				lastFired = t.UTC().YearDay()
				log.Info("push scheduler: sending daily reminders")
				svc.SendDueReminders(ctx)
			}
		}
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
