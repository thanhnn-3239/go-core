// Command app runs the Employee Management System REST API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go-core/internal/handlers"
	"go-core/internal/middleware"
	"go-core/internal/repositories"
	"go-core/internal/services"
)

const shutdownTimeout = 10 * time.Second

type config struct {
	Port           string
	DBPath         string
	ExportDir      string
	RequestTimeout time.Duration
}

// loadConfig reads the configuration from the environment, falling back to
// the defaults for any variable that is unset or empty.
func loadConfig(getenv func(string) string) (config, error) {
	or := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}
	cfg := config{
		Port:      or("PORT", "8080"),
		DBPath:    or("DB_PATH", "data/app.db"),
		ExportDir: or("EXPORT_DIR", "exports"),
	}
	d, err := time.ParseDuration(or("REQUEST_TIMEOUT", "5s"))
	if err != nil {
		return config{}, fmt.Errorf("invalid REQUEST_TIMEOUT: %w", err)
	}
	if d <= 0 {
		return config{}, fmt.Errorf("invalid REQUEST_TIMEOUT: %s must be positive", d)
	}
	cfg.RequestTimeout = d
	return cfg, nil
}

// run wires the application together, serves until ctx is cancelled and then
// shuts the server down gracefully. Resources are released in reverse order
// of acquisition.
func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	db, err := repositories.OpenSQLite(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	empRepo, err := repositories.NewEmployeeSQLite(ctx, db)
	if err != nil {
		return fmt.Errorf("employee repository: %w", err)
	}
	defer empRepo.Close()

	deptRepo, err := repositories.NewDepartmentSQLite(ctx, db)
	if err != nil {
		return fmt.Errorf("department repository: %w", err)
	}
	defer deptRepo.Close()

	router := handlers.NewRouter(
		handlers.NewEmployeeHandler(services.NewEmployeeService(empRepo, deptRepo)),
		handlers.NewDepartmentHandler(services.NewDepartmentService(deptRepo)),
		handlers.NewExportHandler(services.NewExportService(empRepo, cfg.ExportDir)),
	)
	users := map[string]string{"admin": "admin123"}
	handler := middleware.Chain(router,
		middleware.Logging(logger),
		middleware.Recover(logger),
		middleware.Auth(users, "/health"),
		middleware.Timeout(cfg.RequestTimeout),
	)

	srv := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	// utils.WriteError logs 5xx causes through the default logger.
	slog.SetDefault(logger)
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("server failed", "error", err)
		stop()
		os.Exit(1)
	}
}
