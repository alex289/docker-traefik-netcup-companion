package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alex289/docker-traefik-netcup-companion/internal/config"
	"github.com/alex289/docker-traefik-netcup-companion/internal/dns"
	"github.com/alex289/docker-traefik-netcup-companion/internal/docker"
	"github.com/alex289/docker-traefik-netcup-companion/internal/state"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Docker Traefik Netcup Companion...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if cfg.DryRun {
		log.Println("DRY RUN MODE ENABLED - No actual DNS changes will be made")
	}

	// Initialize state manager if persistence is enabled
	var stateManager *state.Manager
	if cfg.StatePersistenceEnabled {
		stateManager, err = state.NewManager(cfg.StateFilePath)
		if err != nil {
			log.Printf("Warning: Failed to initialize state manager: %v", err)
			log.Println("Continuing without state persistence")
		} else {
			log.Printf("State persistence enabled, using file: %s", cfg.StateFilePath)
		}
	} else {
		log.Println("State persistence disabled")
	}

	// Create DNS manager
	dnsManager := dns.NewManager(cfg, stateManager)

	// Create Docker watcher
	watcher, err := docker.NewWatcher(cfg.DockerFilterLabel)
	if err != nil {
		log.Fatalf("Failed to create Docker watcher: %v", err)
	}
	defer watcher.Close()

	// Create context that listens for shutdown signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Perform startup reconciliation if enabled
	if cfg.ReconciliationEnabled && stateManager != nil && stateManager.HasRecords() {
		log.Println("Performing startup reconciliation...")
		if err := dnsManager.ReconcileFromState(ctx); err != nil {
			log.Printf("Warning: Reconciliation failed: %v", err)
		}
	}

	// Scan existing containers first
	log.Println("Scanning existing containers...")
	existingHosts, err := watcher.ScanExistingContainers(ctx)
	if err != nil {
		log.Printf("Warning: Failed to scan existing containers: %v", err)
	} else {
		log.Printf("Found %d existing hosts with Traefik labels", len(existingHosts))
		for _, host := range existingHosts {
			if err := dnsManager.ProcessHostInfo(ctx, host); err != nil {
				log.Printf("Error processing existing host %s: %v", host.Hostname, err)
			}
		}
	}

	// Create channel for host info
	hostChan := make(chan docker.HostInfo, 100)

	// Start goroutine to process host info
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case info := <-hostChan:
				if err := dnsManager.ProcessHostInfo(ctx, info); err != nil {
					log.Printf("Error processing host %s: %v", info.Hostname, err)
				}
			}
		}
	}()

	// Watch for Docker events
	log.Println("Watching for Docker container start events...")
	if err := watcher.WatchEvents(ctx, hostChan); err != nil {
		if ctx.Err() == nil {
			log.Fatalf("Error watching Docker events: %v", err)
		}
	}

	log.Println("Shutdown complete")
}
