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

	"nft-auction-homework/backend/internal/alchemy"
	"nft-auction-homework/backend/internal/api"
	"nft-auction-homework/backend/internal/config"
	"nft-auction-homework/backend/internal/db"
	"nft-auction-homework/backend/internal/indexer"
	"nft-auction-homework/backend/internal/repository"
	"nft-auction-homework/backend/internal/service"

	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err := db.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("connect mysql: %v", err)
	}

	if err := db.RunMigrations(database, cfg.MigrationsDir); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	ethClient, err := ethclient.Dial(cfg.RPCHTTPURL)
	if err != nil {
		log.Fatalf("dial rpc: %v", err)
	}
	defer ethClient.Close()

	repo := repository.New(database)
	alchemyClient := alchemy.NewClient(cfg.AlchemyAPIKey, cfg.AlchemyNetwork)
	svc := service.New(repo, alchemyClient, ethClient, cfg)
	handler := api.NewHandler(svc)
	router := api.NewRouter(handler)

	idx, err := indexer.New(cfg, repo, ethClient)
	if err != nil {
		log.Fatalf("init indexer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go idx.Run(ctx)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.ServerPort),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("backend server listening on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
}
