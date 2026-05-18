package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aditya-mitra/questionnairebot/internal/config"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/scheduler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %s\n", err)
		os.Exit(1)
	}

	log.Printf("Loaded configuration: chat_id=%d data_dir=%s", cfg.ChatID, cfg.DataDir)

	questionnaires, err := loader.Load(cfg.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %s\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	handler := func(slug string) {
		log.Printf("cron fire (stub): %s", slug)
	}
	sched, err := scheduler.New(questionnaires, handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %s\n", err)
		os.Exit(1)
	}
	sched.Start(ctx)

	<-ctx.Done()
	sched.Stop()
	log.Println("shutdown complete")
}
