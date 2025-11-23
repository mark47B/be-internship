package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"

	"github.com/mark47B/be-internship/internal/app"
	"github.com/mark47B/be-internship/internal/configs"
	"github.com/mark47B/be-internship/internal/infra/storage/pg"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/gen"
	"github.com/mark47B/be-internship/internal/infra/transport/rest/handlers"
)

func main() {
	cfg := configs.Load()

	// Connect to database
	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close db: %v", err)
		}
	}()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Initialize repositories
	teamRepo := pg.NewTeamStorage(db)
	userRepo := pg.NewUserStorage(db)
	prRepo := pg.NewPullRequestStorage(db)
	txRepo := pg.NewTxManager(db)

	// Initialize service
	svc := app.NewService(teamRepo, userRepo, prRepo, txRepo)

	// Initialize handlers
	h := handlers.NewHandlers(svc)

	// Setup router
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	// Register handlers
	gen.HandlerFromMux(h, router)

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server
	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
