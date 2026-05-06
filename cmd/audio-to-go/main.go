package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/dawreen/audio-to-go/internal/bot"
	"github.com/dawreen/audio-to-go/internal/config"
	"github.com/dawreen/audio-to-go/internal/gemini"
	"github.com/dawreen/audio-to-go/internal/gitops"
	"github.com/dawreen/audio-to-go/internal/notes"
)

func main() {
	// Load .env if present (dev convenience; in Docker env vars come from env_file).
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf("no .env file: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("bot init error: %v", err)
	}

	ai, err := gemini.New(cfg)
	if err != nil {
		log.Fatalf("gemini init error: %v", err)
	}

	if err := gitops.InitRepo(cfg.GitRemoteURL, cfg.GitUserEmail, cfg.GitUserName, cfg.NotesDir); err != nil {
		log.Fatalf("failed to init git repository: %v", err)
	}

	// Setup graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Notify the user that the bot is up
	b.Reply(cfg.AllowedUserID, "🚀 Bot is up and ready to receive notes!")

	// Processor goroutine — single consumer, serializes all file writes and git ops.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for job := range b.Jobs() {
			processJob(b, ai, cfg, job)
		}
		log.Println("Processor finished all jobs.")
	}()

	if err := b.Run(ctx); err != nil {
		log.Fatalf("bot stopped: %v", err)
	}

	// Wait for processor to drain the queue
	<-done
	
	// Notify the user that the bot is down
	b.Reply(cfg.AllowedUserID, "💤 Bot has shut down gracefully.")
	log.Println("Graceful shutdown complete.")
}

func processJob(b *bot.Bot, ai *gemini.Client, cfg *config.Config, job bot.AudioJob) {
	log.Printf("processing audio fileID=%s mimeType=%s", job.FileID, job.MIMEType)

	audioBytes, err := b.DownloadFile(job.FileID)
	if err != nil {
		log.Printf("download error: %v", err)
		b.Reply(job.ChatID, "Failed to download audio — try again.")
		return
	}
	log.Printf("downloaded %d bytes", len(audioBytes))

	markdown, err := ai.Analyze(context.Background(), audioBytes, job.MIMEType)
	if err != nil {
		log.Printf("gemini error: %v", err)
		b.Reply(job.ChatID, "Failed to analyze audio — try again.")
		return
	}
	log.Printf("gemini response:\n%s", markdown)

	if err := notes.Append(cfg.NotesDir, markdown); err != nil {
		log.Printf("notes error: %v", err)
		b.Reply(job.ChatID, "Failed to write note — try again.")
		return
	}

	if err := gitops.Push(cfg.NotesDir, cfg.GitRemoteURL); err != nil {
		log.Printf("git push error: %v", err)
		b.Reply(job.ChatID, "Note saved but push failed — will sync on next note.\n\n"+markdown)
		return
	}

	b.Reply(job.ChatID, "Done! Note saved and pushed.\n\n"+markdown)
}
