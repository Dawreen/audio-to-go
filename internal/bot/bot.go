package bot

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/dawreen/audio-to-go/internal/config"
)

// AudioJob holds everything needed to process a single audio note.
type AudioJob struct {
	FileID   string
	ChatID   int64
	MIMEType string // e.g. "audio/ogg" for voice, "audio/mp4" for m4a
}

// Bot wraps the Telegram client and the processing queue.
type Bot struct {
	api      *tgbotapi.BotAPI
	cfg      *config.Config
	jobQueue chan AudioJob
}

func New(cfg *config.Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("connecting to Telegram: %w", err)
	}
	log.Printf("Authorized as @%s", api.Self.UserName)

	return &Bot{
		api:      api,
		cfg:      cfg,
		jobQueue: make(chan AudioJob, 10),
	}, nil
}

// Jobs returns the read-only job channel for the processor goroutine.
func (b *Bot) Jobs() <-chan AudioJob {
	return b.jobQueue
}

// Reply sends a text message back to a chat.
func (b *Bot) Reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("reply error: %v", err)
	}
}

// DownloadFile fetches a Telegram file and returns its raw bytes.
func (b *Bot) DownloadFile(fileID string) ([]byte, error) {
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, fmt.Errorf("getting file info: %w", err)
	}
	url := file.Link(b.cfg.TelegramToken)
	resp, err := http.Get(url) //nolint:gosec // URL is from Telegram's own API
	if err != nil {
		return nil, fmt.Errorf("downloading file: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Run starts the long-polling loop. Blocks until the context is canceled.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)
	log.Println("Polling for updates...")

	// Listen for context cancellation to stop updates gracefully
	go func() {
		<-ctx.Done()
		log.Println("Context canceled, stopping Telegram updates...")
		b.api.StopReceivingUpdates()
	}()

	for update := range updates {
		if update.Message == nil {
			continue
		}
		b.handleMessage(update.Message)
	}

	// After the updates channel is closed and all current messages are handled, close the job queue
	close(b.jobQueue)
	return nil
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	// Drop messages from anyone other than the authorized user.
	if msg.From.ID != b.cfg.AllowedUserID {
		return
	}

	var job AudioJob
	job.ChatID = msg.Chat.ID

	switch {
	case msg.Voice != nil:
		// In-app voice recording — always OGG Opus.
		job.FileID = msg.Voice.FileID
		job.MIMEType = "audio/ogg"

	case msg.Audio != nil:
		// Audio file sent from phone (e.g. .m4a recordings).
		job.FileID = msg.Audio.FileID
		job.MIMEType = mimeForAudio(msg.Audio.MimeType)

	default:
		b.Reply(msg.Chat.ID, "Send me a voice note or audio recording and I'll turn it into a structured note.")
		return
	}

	// Non-blocking send: if the queue is full, tell the user instead of blocking.
	select {
	case b.jobQueue <- job:
		b.Reply(msg.Chat.ID, "Got it, processing...")
	default:
		b.Reply(msg.Chat.ID, "Queue is full — try again in a moment.")
	}
}

// mimeForAudio maps Telegram's reported MIME type to what Gemini expects.
// Falls back to audio/mp4 which covers m4a/aac files.
func mimeForAudio(telegramMIME string) string {
	switch telegramMIME {
	case "audio/ogg":
		return "audio/ogg"
	case "audio/mpeg", "audio/mp3":
		return "audio/mp3"
	case "audio/wav":
		return "audio/wav"
	case "audio/flac":
		return "audio/flac"
	case "audio/webm":
		return "audio/webm"
	default:
		// m4a files report as audio/mp4 or audio/x-m4a — Gemini accepts audio/mp4.
		return "audio/mp4"
	}
}
