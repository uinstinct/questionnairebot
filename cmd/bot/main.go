package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aditya-mitra/questionnairebot/internal/bot"
	"github.com/aditya-mitra/questionnairebot/internal/commands"
	"github.com/aditya-mitra/questionnairebot/internal/config"
	"github.com/aditya-mitra/questionnairebot/internal/handler"
	"github.com/aditya-mitra/questionnairebot/internal/loader"
	"github.com/aditya-mitra/questionnairebot/internal/scheduler"
	"github.com/aditya-mitra/questionnairebot/internal/session"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	log.Printf("Loaded configuration: chat_id=%d data_dir=%s", cfg.ChatID, cfg.DataDir)

	questionnaires, err := loader.Load(cfg.DataDir)
	if err != nil {
		fatal(err)
	}

	sessions := session.NewManager(cfg.DataDir)
	flow := handler.New(nil, sessions, cfg.DataDir, questionnaires)
	disp := handler.NewDispatcher(flow)

	b, err := bot.New(cfg.BotToken, cfg.ChatID, disp)
	if err != nil {
		fatal(err)
	}
	flow.Sender = b

	if err := handler.Restore(flow); err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bus := commands.NewCronBus(flow, b, time.Now)
	go bus.Run(ctx)

	pull := commands.NewPull(flow, time.Now)
	status := commands.NewStatus(cfg.DataDir, sessions, flow.Questionnaires, time.Now)
	list := commands.NewList(flow.Questionnaires, time.Now)
	disp.Attach(commands.NewAdapter(pull, status, list))

	sched, err := scheduler.New(questionnaires, func(slug string) { bus.Fire(slug, time.Now()) })
	if err != nil {
		fatal(err)
	}
	sched.Start(ctx)

	go b.Run(ctx)

	<-ctx.Done()
	sched.Stop()
	log.Println("shutdown complete")
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "FATAL: %s\n", err)
	os.Exit(1)
}
