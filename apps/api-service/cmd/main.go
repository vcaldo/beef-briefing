// Placeholder for API Service main entry point
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a context that cancels on SIGINT or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Start your services/goroutines here
	log.Println("Service started")

	// Block until signal received
	<-ctx.Done()

	// Cleanup happens here
	log.Println("Shutting down gracefully...")
	// Close connections, flush buffers, etc.

	log.Println("Shutdown complete")
}
