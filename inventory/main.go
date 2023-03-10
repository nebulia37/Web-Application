package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"cse5234/inventory/pkg/config"
	"cse5234/inventory/pkg/mysql"
	"cse5234/inventory/pkg/routes"
)

func main() {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// get a copy of configuations
	options := config.NewConfig()
	if options == nil {
		panic(fmt.Errorf("invalid configuation"))
	}

	// Set up database connection
	db, err := mysql.Connect(options.Database.User, options.Database.Password, options.Database.Name, options.Database.Address)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Run migrations
	if err := mysql.Migrate(db); err != nil {
		panic(err)
	}

	// Set up database service
	dbService := mysql.NewService(db)
	if dbService == nil {
		panic(fmt.Errorf("cannot create DB Service"))
	}

	// Create a new handler
	handler := routes.NewHandler(dbService)

	serverAddr := fmt.Sprintf(":%d", options.Port)
	log.Printf("Listening on %s", serverAddr)

	// Create an HTTP server
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: handler.Routes(),
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Listen for the interrupt signal.
	<-ctx.Done()

	// Restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
