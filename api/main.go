package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joe-ervin05/atomicbase/config"
	"github.com/joe-ervin05/atomicbase/data"
	"github.com/joe-ervin05/atomicbase/platform"
	"github.com/joe-ervin05/atomicbase/tools"
)

func logStartupInfo() {
	fmt.Println("=== Atomicbase ===")
	fmt.Printf("Port:            %s\n", config.Cfg.Port)
	fmt.Printf("Database:        %s\n", config.Cfg.PrimaryDBPath)
	fmt.Printf("Request timeout: %ds\n", config.Cfg.RequestTimeout)
	fmt.Printf("Query depth:     %d max\n", config.Cfg.MaxQueryDepth)
	fmt.Printf("Pagination:      %d default, %d max\n", config.Cfg.DefaultLimit, config.Cfg.MaxQueryLimit)

	// Security warnings
	warnings := 0
	if config.Cfg.APIKey == "" {
		fmt.Println("[WARN] No API key set - authentication disabled")
		warnings++
	} else {
		fmt.Println("[OK]   Authentication enabled")
	}

	if !config.Cfg.RateLimitEnabled {
		fmt.Println("[WARN] Rate limiting disabled")
		warnings++
	} else {
		fmt.Printf("[OK]   Rate limiting: %d req/min\n", config.Cfg.RateLimit)
	}

	if len(config.Cfg.CORSOrigins) == 0 {
		fmt.Println("[INFO] CORS disabled (no origins configured)")
	} else {
		fmt.Printf("[OK]   CORS origins: %v\n", config.Cfg.CORSOrigins)
	}

	if warnings > 0 {
		fmt.Printf("\n[!] %d security warning(s) - review before production\n", warnings)
	}
	fmt.Println()
}

func main() {
	logStartupInfo()

	app := http.NewServeMux()

	// Register routes from each module
	data.RegisterRoutes(app)
	platform.RegisterRoutes(app)

	// Apply middleware chain: panic recovery -> logging -> timeout -> cors -> rate limit -> auth -> handler
	handler := tools.PanicRecoveryMiddleware(
		tools.LoggingMiddleware(
			tools.TimeoutMiddleware(
				tools.CORSMiddleware(
					tools.RateLimitMiddleware(
						tools.AuthMiddleware(app))))))

	server := &http.Server{
		Addr:    config.Cfg.Port,
		Handler: handler,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("Listening on %s\n", config.Cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down server...")

	// Give outstanding requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Close the database connection
	if err := data.ClosePrimaryDB(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	fmt.Println("Server stopped")
}
