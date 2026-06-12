// Command bot runs the Telegram questionnaire bot binary.
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
	"github.com/aditya-mitra/questionnairebot/internal/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	log.Printf("Loaded configuration: chat_id=%d data_dir=%s", cfg.ChatID, cfg.DataDir)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Opt-in telemetry: when no OTLP endpoint is configured this is a no-op and
	// the bot behaves identically. Setup failures are downgraded to a WARN + a
	// no-op shutdown so telemetry can never block startup.
	shutdownTel, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		log.Printf("WARN: telemetry setup: %v", err)
		shutdownTel = func(context.Context) error { return nil }
	}

	questionnaires, err := loader.Load(cfg.DataDir)
	if err != nil {
		fatal(err)
	}

	sessions := session.NewManager(cfg.DataDir)
	telemetry.SetActiveSessionsSource(func() int64 { return int64(sessions.Len()) })
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

	if err := b.RegisterCommands(commands.Commands()); err != nil {
		log.Printf("WARN: failed to register bot commands: %v", err)
	}

	go b.Run(ctx)

	<-ctx.Done()
	sched.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdownTel(shutdownCtx); err != nil {
		log.Printf("WARN: telemetry shutdown: %v", err)
	}
	log.Println("shutdown complete")
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "FATAL: %s\n", err)
	os.Exit(1)
}
