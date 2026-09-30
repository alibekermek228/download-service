package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"

	"internship-download-service/internal/config"
	"internship-download-service/internal/downloader"
	postgresrepo "internship-download-service/internal/repository/postgres"
	"internship-download-service/internal/temporalapp"
	httptransport "internship-download-service/internal/transport/http"
	"internship-download-service/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	appContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	pool, err := connectPostgres(appContext, cfg.DatabaseURL, 30*time.Second)
	if err != nil {
		return err
	}
	defer pool.Close()

	temporalClient, err := connectTemporal(appContext, cfg.TemporalAddress, 30*time.Second)
	if err != nil {
		return err
	}
	defer temporalClient.Close()

	repository := postgresrepo.NewDownloadRepository(pool)
	fileDownloader := downloader.NewHTTP(cfg.MaxFileSizeBytes)
	activities := temporalapp.NewActivities(
		repository,
		fileDownloader,
		cfg.MaxConcurrentDownloads,
	)
	temporalWorker := temporalapp.NewWorker(
		temporalClient,
		cfg.TemporalTaskQueue,
		activities,
	)
	if err := temporalWorker.Start(); err != nil {
		return fmt.Errorf("start Temporal worker: %w", err)
	}

	starter := temporalapp.NewStarter(temporalClient, cfg.TemporalTaskQueue)
	service := usecase.New(repository, starter)
	handler := httptransport.NewHandler(service)
	server := &http.Server{
		Addr:              cfg.APIAddress,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("HTTP server is listening on %s", cfg.APIAddress)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-appContext.Done():
		log.Print("shutdown signal received")
	case serverErr := <-serverErrors:
		if !errors.Is(serverErr, http.ErrServerClosed) {
			stop()
			log.Printf("HTTP server stopped: %v", serverErr)
		}
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	workerStopped := make(chan struct{})
	go func() {
		temporalWorker.Stop()
		close(workerStopped)
	}()
	select {
	case <-workerStopped:
		log.Print("service stopped gracefully")
	case <-shutdownContext.Done():
		panic("graceful shutdown exceeded its timeout")
	}

	return nil
}

func connectPostgres(ctx context.Context, databaseURL string, timeout time.Duration) (*pgxpool.Pool, error) {
	connectContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		pool, err := pgxpool.New(connectContext, databaseURL)
		if err == nil {
			err = pool.Ping(connectContext)
		}
		if err == nil {
			return pool, nil
		}
		if pool != nil {
			pool.Close()
		}

		select {
		case <-connectContext.Done():
			return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
		case <-time.After(time.Second):
		}
	}
}

func connectTemporal(ctx context.Context, address string, timeout time.Duration) (client.Client, error) {
	connectContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		temporalClient, err := client.DialContext(connectContext, client.Options{HostPort: address})
		if err == nil {
			return temporalClient, nil
		}

		select {
		case <-connectContext.Done():
			return nil, fmt.Errorf("connect to Temporal: %w", err)
		case <-time.After(time.Second):
		}
	}
}
