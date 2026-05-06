# Audio-to-Go

A Telegram bot built in Go that receives voice messages, uses Google Gemini to transcribe and analyze the audio into markdown notes, and automatically commits those notes to a remote Git repository.

## Features

- **Telegram Bot Integration:** Only accepts messages from a specific allowed user ID to ensure privacy.
- **Audio Analysis:** Leverages Google's Gemini API (default: `gemini-3-pro-preview`) to accurately transcribe and format voice notes.
- **GitOps Flow:** Automatically initializes, commits, and pushes the resulting Markdown files to a secure remote Git repository over SSH.
- **Dockerized:** Ready to be deployed via Docker Compose to a homelab or VPS (e.g., via Portainer).

## Tech Stack

- **Language:** Go 1.24
- **AI Integration:** Google Gemini API
- **Containerization:** Docker & Docker Compose
- **Version Control:** Git & SSH for secure pushes

## Prerequisites

- Go 1.24+ (for local development)
- Docker & Docker Compose
- A Telegram Bot Token (from [@BotFather](https://t.me/botfather))
- A Gemini API Key (from [Google AI Studio](https://aistudio.google.com/))
- A remote Git repository configured for SSH access (with the deploy key mounted into the container or your local agent setup)

## Configuration

Copy the example environment file and configure it:

```bash
cp .env.example .env
```

Update your `.env` file with the required variables:

```env
TELEGRAM_TOKEN=your-telegram-bot-token
ALLOWED_USER_ID=123456789             # Your Telegram user ID
GEMINI_API_KEY=your-gemini-api-key
GEMINI_MODEL=gemini-2.0-flash         # Optional: defaults to gemini-2.0-flash
GIT_REMOTE_URL=git@github.com:youruser/notes-repo.git
GIT_USER_EMAIL=your@email.com
GIT_USER_NAME=Your Name
TZ=Europe/Rome                        # Timezone for note timestamps
```

## Running Locally

1. Install dependencies:
   ```bash
   go mod download
   ```
2. Build and run:
   ```bash
   go build -o audio-to-go ./cmd/audio-to-go
   ./audio-to-go
   ```

## Running with Docker / Portainer

You can spin up the application using Docker Compose. Make sure your SSH keys are correctly mapped into the container so it can push to the remote Git repository.

```bash
# Standard Docker Compose deployment
docker-compose up -d --build
```

If deploying via **Portainer**, you can use the provided `docker-compose.portainer.yml` stack definition.

## Project Structure

- `cmd/audio-to-go/`: Main entry point for the application.
- `internal/bot/`: Telegram bot initialization and message handling.
- `internal/config/`: Configuration and environment variable loading.
- `internal/gemini/`: Google Gemini API client for audio analysis.
- `internal/gitops/`: Git initialization, commit, and push operations.
- `internal/notes/`: Markdown note generation and file system operations.
