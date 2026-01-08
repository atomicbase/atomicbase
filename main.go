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

	"github.com/joe-ervin05/atomicbase/api"
	"github.com/joe-ervin05/atomicbase/config"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
}

func main() {
	app := http.NewServeMux()

	api.Run(app)

	// Apply middleware chain: timeout -> cors -> rate limit -> auth -> handler
	handler := api.TimeoutMiddleware(
		api.CORSMiddleware(
			api.RateLimitMiddleware(
				api.AuthMiddleware(app))))

	server := &http.Server{
		Addr:    config.Cfg.Port,
		Handler: handler,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("Listening on port %s\n", config.Cfg.Port)
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

	fmt.Println("Server stopped")
}
