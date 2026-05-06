# audio-to-go — Complete Project Walkthrough

> A learning guide for a data engineer new to Go and Docker.
> Covers every file in the project with explanations of Go concepts, patterns, and design decisions.

---

## Project Overview

`audio-to-go` is a Telegram bot that:
1. Receives voice notes or audio files from you
2. Sends the audio to Google Gemini AI for transcription and structuring
3. Appends the result as a markdown note to a daily file
4. Commits and pushes the note to a private Git repository

---

## Project Structure

```
audio_to_go/
├── cmd/audio-to-go/main.go          ← entry point, wires everything together
├── internal/
│   ├── config/config.go             ← loads env vars into a typed struct
│   ├── bot/bot.go                   ← Telegram bot, message handling, job queue
│   ├── gemini/gemini.go             ← Gemini AI client, audio analysis
│   ├── notes/notes.go               ← writes markdown notes to disk
│   └── gitops/gitops.go             ← git init, commit, push logic
├── go.mod / go.sum                  ← module definition and dependency lock
├── .env / .env.example              ← secrets (ignored by Git) / template
├── Dockerfile                       ← multi-stage build
├── docker-compose.yml               ← local development
├── docker-compose.portainer.yml     ← production (Portainer)
├── Makefile                         ← developer shortcuts
└── .gitignore                       ← files Git never tracks
```

---

## File 1: `go.mod`

**What it is:** Go's module manifest — equivalent to `package.json` (Node) or `requirements.txt` (Python).

```go
module github.com/dawreen/audio-to-go  // unique module name (URL-style by convention)
go 1.24                                // minimum Go version required

require (
    github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1  // Telegram SDK
    github.com/joho/godotenv v1.5.1                             // reads .env files
    google.golang.org/genai v1.56.0                             // Gemini AI SDK
)
```

**Key concepts:**
- The module path (e.g. `github.com/dawreen/audio-to-go`) is the **identity** of the project. Other files use it in import paths.
- `go.sum` contains cryptographic hashes of all dependencies — guarantees reproducible builds. Always commit it.
- `go.mod` is **auto-generated**: `go mod init <name>` creates it, `go mod tidy` keeps it in sync.

**Creating a new project:**
```bash
mkdir my-project && cd my-project
git init
go mod init github.com/yourname/my-project
# write code, add imports
go mod tidy   # downloads and records all dependencies
```

---

## File 2: `.env.example`

**What it is:** A committed template documenting every environment variable the app needs. The actual `.env` file is in `.gitignore` and never committed.

```
TELEGRAM_TOKEN=your-telegram-bot-token
ALLOWED_USER_ID=123456789
GEMINI_API_KEY=your-gemini-api-key
GEMINI_MODEL=gemini-2.0-flash
GIT_REMOTE_URL=git@github.com:youruser/notes.git
GIT_USER_EMAIL=your@email.com
GIT_USER_NAME=Your Name
TZ=Europe/Rome
```

**Security model:**
```
.env.example  → committed to Git   ✅ (safe, fake values)
.env          → ignored by Git     ✅ (real secrets, stays local)
```

**Usage:**
```bash
cp .env.example .env
# fill in real values
```

---

## File 3: `internal/config/config.go`

**What it does:** Reads all environment variables and packs them into a single typed `Config` struct used by every other package.

```go
type Config struct {
    TelegramToken string
    AllowedUserID int64   // int64 because Telegram IDs are large numbers
    GeminiAPIKey  string
    GeminiModel   string
    GitRemoteURL  string
    GitUserEmail  string
    GitUserName   string
    NotesDir      string
}

func Load() (*Config, error) { ... }
```

**Key Go concepts:**

**Structs** — bundles of named fields with types. Like a row in a table.

**Multiple return values** — Go's most distinctive feature. Functions return `(result, error)`. The caller checks the error immediately.
```go
cfg, err := config.Load()
if err != nil {
    log.Fatalf("config error: %v", err)
}
```

**`os.Getenv()`** — reads an environment variable. Returns `""` if not set (no panic, no exception).

**Why env vars instead of reading a file?**
- Works the same way locally, in Docker, in Portainer, in Kubernetes
- Secrets are injected by the runtime — never baked into the image
- Follows the [12-Factor App](https://12factor.net/) principle: *store config in the environment*

**`strconv.ParseInt(rawUID, 10, 64)`** — converts a string to `int64`. Base 10, 64-bit. Required because env vars are always strings.

**The `internal/` directory rule:** Packages in `internal/` can only be imported by code within the same module. It signals "private implementation, not a public API."

---

## File 4: `internal/bot/bot.go`

**What it does:** Wraps the Telegram API, manages the job queue, handles messages, and enforces security.

### Pointers: `&` and `*`

```go
// & = "give me the address of"
x := 42
p := &x           // p holds the memory address of x

// * in a type = "this is a pointer"
var b *Bot        // b holds an address of a Bot, not the Bot itself

// * in an expression = "go to that address and get the value"
fmt.Println(*p)   // → 42
```

**Why use pointers?**
1. **Efficiency** — pass 8 bytes (address) instead of copying the entire struct
2. **Shared mutation** — changes inside a function affect the original

### Channels

A channel is a **typed, thread-safe pipe** between goroutines (lightweight threads).

```go
jobQueue := make(chan AudioJob, 10)  // buffered channel, holds up to 10 jobs

// SEND — put a job in
jobQueue <- job

// RECEIVE — take a job out (blocks until available)
job := <-jobQueue

// RANGE — loop until channel is closed
for job := range jobQueue { ... }

// CLOSE — signal "no more data coming"
close(jobQueue)
```

**Buffered vs unbuffered:**
- `make(chan T)` → sender blocks until receiver reads (synchronous)
- `make(chan T, 10)` → sender can put 10 items before blocking

**Read-only channel type:** `<-chan AudioJob` — the receiver can only read, not write. Enforced by the compiler.

**`select` for non-blocking sends:**
```go
select {
case b.jobQueue <- job:
    b.Reply(msg.Chat.ID, "Got it, processing...")
default:                    // runs if channel is full, instead of blocking
    b.Reply(msg.Chat.ID, "Queue is full — try again.")
}
```

### Other key concepts

**Capitalization = access control:**
- `Uppercase` fields/functions → public (any package can use)
- `lowercase` fields/functions → private (only this package)

**Methods (receiver functions):**
```go
func (b *Bot) Reply(chatID int64, text string) { ... }
// (b *Bot) = receiver, like "self" in Python
```

**`defer`** — schedules a call to run when the function returns, no matter what:
```go
defer resp.Body.Close()  // always releases the HTTP connection
```

**Graceful shutdown with context:**
```go
go func() {
    <-ctx.Done()                    // waits for cancellation signal
    b.api.StopReceivingUpdates()    // then stops the bot
}()
```

---

## File 5: `internal/gemini/gemini.go`

**What it does:** Sends audio bytes + a prompt to Gemini AI and returns structured markdown.

### Key Go concepts

**`const` with raw string literals:**
```go
const prompt = `...multiline string...
%s is a format placeholder
`
// Backticks = raw strings: newlines, tabs, special chars all literal
```

**`context.Context`** — travels through your entire call chain, carries cancellation signals and deadlines.
- `context.Background()` → root context, never cancels. Use for initialization.
- Propagate the caller's `ctx` in regular functions so callers can cancel operations.

```
context.Background()           ← root
    └─ signal.NotifyContext()  ← cancels on Ctrl+C / SIGTERM
           └─ b.Run(ctx)       ← bot stops polling
                  └─ ai.Analyze(ctx, ...) ← HTTP call is cancelled
```

**Go's time format reference time:** `Mon Jan 2 15:04:05 MST 2006`
```go
time.Now().Format("15:04")       // → "14:32"  (hour:minute)
time.Now().Format("2006-01-02")  // → "2026-05-06"  (ISO date)
// 15=hour, 04=minute, 2006=year, 01=month, 02=day
// Memory trick: 1 2 3 4 5 6 7 (month day hour minute second year timezone)
```

**`[]byte`** — a slice of bytes. Raw binary data (audio files, file contents) is always `[]byte`.

**Exponential backoff retry:**
```go
backoff := 2 * time.Second
for attempt := 1; attempt <= 3; attempt++ {
    result, err := callAPI()
    if err != nil {
        time.Sleep(backoff)
        backoff *= 2  // 2s → 4s → 8s
        continue
    }
    return result, nil
}
```
Handles transient network errors and API rate limits gracefully.

**String slicing:**
```go
raw[start:]      // from index 'start' to end
raw[:end+3]      // from beginning to index 'end+3'
raw[start:end]   // from 'start' to 'end'
```

---

## File 6: `internal/notes/notes.go`

**What it does:** Creates daily markdown files and appends note blocks to them.

Result on disk:
```
notes/
├── 2026-05-05.md
└── 2026-05-06.md    ← contains all notes from today
```

Each file:
```markdown
# 2026-05-06

## Meeting Note — 09:32
...

## Meeting Note — 14:17
...
```

**Key concepts:**

**`os.MkdirAll(path, 0o755)`** — creates directory + all missing parents (like `mkdir -p`)

**Unix permissions (octal):**
```
0o755 = rwxr-xr-x  (directories need execute to allow "entering")
0o644 = rw-r--r--  (files: owner read/write, others read-only)
```

**`filepath.Join()`** — always use this, never string concatenation. Works correctly on all OS (handles `/` vs `\`).

**`os.Stat()` + `os.IsNotExist(err)`** — standard pattern to check if a file exists:
```go
if _, err := os.Stat(path); os.IsNotExist(err) {
    // file doesn't exist, create it
}
```

**`os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)`**
- `O_APPEND` — all writes go to the end of the file
- `O_WRONLY` — open for writing only
- `|` is bitwise OR — combines flags (each is a power of 2)

**`fmt.Fprintf(f, "\n%s\n", block)`** — writes formatted string directly to any `io.Writer` (file, network connection, buffer — same interface).

---

## File 7: `internal/gitops/gitops.go`

**What it does:** Replaces what used to be a shell script — sets up the git repo on first run and commits/pushes after every note.

### `runIn()` — the foundation

```go
func runIn(dir, name string, args ...string) error {
    cmd := exec.Command(name, args...)  // create the command
    cmd.Dir = dir                       // set working directory
    out, err := cmd.CombinedOutput()    // run it, capture stdout+stderr
    if err != nil {
        return fmt.Errorf("%w — %s", err, strings.TrimSpace(string(out)))
    }
    return nil
}
```

**`args ...string`** — variadic parameter: accepts zero or more strings. Inside the function it's a `[]string`. When calling: `runIn("/app", "git", "add", ".")` — the strings after `"git"` become `args`.

**`args...`** — unpacks a slice back into individual arguments when calling another variadic function.

**Why shell out to `git` instead of using a Go library?** The `git` binary is already in the Docker image, battle-tested, and produces readable error messages. For simple automation, this is pragmatic and sufficient.

### `Push()` strategy

```go
// 1. Stage all changes
runIn(notesDir, "git", "add", ".")

// 2. Commit with timestamp
runIn(notesDir, "git", "commit", "-m", "note: 2026-05-06 14:32")

// 3. Pull remote changes BEFORE pushing (avoids rejection)
_ = runIn(notesDir, "git", "pull", "--rebase", remoteURL, "main")

// 4. Push
runIn(notesDir, "git", "push", remoteURL, "HEAD:main", "--force-with-lease")
```

- `--rebase` replays your commit on top of remote state → no merge commits
- `--force-with-lease` is safer than `--force` — only overwrites remote if it hasn't changed since your last fetch
- `_ =` deliberately discards the pull error (remote might be empty on first run)

---

## File 8: `cmd/audio-to-go/main.go`

**What it does:** The entry point. Initializes all services and wires them together.

**Why `cmd/audio-to-go/`?** Convention:
- `cmd/` = executable programs
- `internal/` = reusable packages
- Multiple binaries would be `cmd/tool1/`, `cmd/tool2/`, etc.

### Graceful shutdown

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
```

- `os.Interrupt` = Ctrl+C
- `syscall.SIGTERM` = what `docker stop` sends
- When received, `ctx` is cancelled → everything downstream stops cleanly

**`log.Fatalf`** — logs + calls `os.Exit(1)`. Used when the app cannot start.
**`log.Printf`** — logs only, execution continues.

### The processor goroutine pattern

```go
done := make(chan struct{})      // signal-only channel (struct{} = 0 bytes)
go func() {
    defer close(done)           // signals "I'm finished" when the loop exits
    for job := range b.Jobs() {
        processJob(b, ai, cfg, job)
    }
}()

b.Run(ctx)   // blocks here until shutdown signal
<-done       // waits for processor to finish all remaining jobs
```

**Two goroutines working concurrently:**
```
Goroutine 1 (main):       b.Run(ctx) — polls Telegram, puts jobs in channel
Goroutine 2 (processor):  for job := range — processes jobs from channel
```

### The `processJob()` pipeline

```
DownloadFile()  →  ai.Analyze()  →  notes.Append()  →  gitops.Push()  →  Reply()
```

Each step: fail → tell user → `return`. Deliberately ordered: write the note before pushing. If Git is down, the note is still saved locally.

### Complete data flow

```
Voice note on Telegram
  → bot.Run() receives message
  → handleMessage() creates AudioJob
  → jobQueue <- job  (channel)
  → processJob() reads from channel
  → DownloadFile() → []byte audio
  → ai.Analyze()  → markdown string
  → notes.Append() → writes to disk
  → gitops.Push() → commits + pushes to GitHub
  → b.Reply()     → "Done! Note saved."
```

---

## File 9: `Dockerfile`

**What it does:** Defines how to build the Docker image. Uses a **multi-stage build**.

```dockerfile
# STAGE 1: build
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./        # copy dependency manifest FIRST (layer cache optimization)
RUN go mod download           # download deps (cached if go.mod unchanged)
COPY . .                      # copy source code
RUN CGO_ENABLED=0 go build -o audio-to-go ./cmd/audio-to-go

# STAGE 2: runtime
FROM alpine:3.19
RUN apk add --no-cache git openssh tzdata
WORKDIR /app
COPY --from=builder /app/audio-to-go .   # only the binary crosses stages
ENTRYPOINT ["./audio-to-go"]
```

**Multi-stage builds:** Build in a large image (Go compiler ~300MB), copy only the binary to a tiny image (~20MB). Build artifacts and source code are discarded.

**Layer caching:** Docker caches each instruction. Copying `go.mod` before source code means dependency download is only re-run when dependencies change, not every time you edit a `.go` file.

**`CGO_ENABLED=0`** — produces a **fully static binary**. No external runtime dependencies (no glibc, no C compiler needed). The binary runs anywhere.

**`ENTRYPOINT ["./audio-to-go"]` (exec form)** — runs the binary directly as PID 1. Docker's SIGTERM goes straight to your process, enabling graceful shutdown. Shell form would break signal forwarding.

**Why `git`, `openssh`, `tzdata`?**
- `git` + `openssh` — needed for `gitops.go` which calls `exec.Command("git", ...)`
- `tzdata` — without it, `TZ=Europe/Rome` is ignored and all timestamps are UTC

---

## File 10: `docker-compose.yml` (local dev)

```yaml
services:
  audio-to-go:
    build: .                          # build from local Dockerfile
    restart: unless-stopped           # restart on crash/reboot, not on manual stop
    env_file: .env                    # inject all vars from .env file
    volumes:
      - .:/app                        # bind mount: live project dir
      - /root/.ssh:/root/.ssh:ro      # SSH keys for git push (read-only)
```

**Restart policies:**
- `no` — never restart
- `always` — always restart
- `unless-stopped` — restart on crash/reboot, respect manual stop ✅
- `on-failure` — only on non-zero exit

**`env_file: .env`** — reads `.env` and injects all variables into the container's environment.

**Bind mount (`.:/app`)** — mounts host directory directly. Changes on host are instantly visible in container and vice versa. Good for dev, riskier for production.

**`:ro`** — read-only mount. Container can use SSH keys but cannot modify them.

---

## File 11: `docker-compose.portainer.yml` (production)

```yaml
services:
  audio-to-go:
    build:
      context: .
      dockerfile: Dockerfile
    restart: unless-stopped
    environment:
      - TELEGRAM_TOKEN=${TELEGRAM_TOKEN}         # from Portainer UI
      - GEMINI_MODEL=${GEMINI_MODEL:-gemini-2.0-flash}  # with default value
      - TZ=${TZ:-Europe/Rome}
    volumes:
      - notes_data:/app/notes    # named volume: Docker-managed storage

volumes:
  notes_data:                    # declares the named volume
```

**Key differences from local compose:**

| | `docker-compose.yml` | `docker-compose.portainer.yml` |
|---|---|---|
| Secrets source | `env_file: .env` | Portainer UI → `${VAR}` substitution |
| Volume type | Bind mount (host dir) | Named volume (Docker-managed) |
| Scope | Full project dir | Only `/app/notes` |
| Deployment | Manual | Portainer manages |

**Named volumes vs bind mounts:**
- Named volumes are managed by Docker — no host path required
- Survive container recreation
- Portainer can inspect and backup them via UI
- Perfect for production persistent data

**`${VAR:-default}`** — if `VAR` is not set, use `default`. Second layer of safety beyond `config.go`.

---

## File 12: `Makefile`

Developer shortcuts — type `make <target>` instead of long commands.

```makefile
.PHONY: run build clean   # not real files — always execute

run:
    -pkill -f "audio-to-go" 2>/dev/null; sleep 1  # kill previous instance
    CGO_ENABLED=0 go run ./cmd/audio-to-go         # compile + run in one step

build:
    CGO_ENABLED=0 go build -o audio-to-go ./cmd/audio-to-go  # produce binary

clean:
    rm -f audio-to-go    # delete binary
    go clean -cache      # clear Go build cache
```

**Critical:** Makefile indentation must be **TABs**, not spaces.

**`-` prefix** on a command — ignore non-zero exit code, continue.
**`2>/dev/null`** — redirect stderr to the void (suppress error messages).
**`go run`** vs **`go build`** — `run` compiles and executes in one step (no binary produced), great for development.

**Deployment is handled by Portainer UI** — the `deploy` target was removed since Portainer manages environment variables and restarts.

---

## File 13: `.gitignore`

```
.env              # secrets — NEVER commit
/audio-to-go      # compiled binary (leading / = root only)
notes/            # notes have their own separate Git repo
.DS_Store         # macOS Finder metadata, useless to everyone else
```

**Why `notes/` is ignored:** The bot writes notes to `notes/` and `gitops.go` pushes them to a **separate** GitHub repository. If `notes/` weren't ignored, you'd have a Git repo inside a Git repo (a "submodule") — not what you want.

**`go.sum` is NOT ignored** — it must be committed. It's the cryptographic lock that guarantees reproducible builds.

---

## SSH Key Authentication

How `make deploy`, `git push`, and the container authenticate without passwords:

```bash
# 1. Generate key pair (once)
ssh-keygen -t ed25519 -C "your@email.com"
# Creates: ~/.ssh/id_ed25519 (private, never share)
#          ~/.ssh/id_ed25519.pub (public, safe to share)

# 2. Copy public key to server (one-time, asks for password this once)
ssh-copy-id homelab

# 3. From now on, password-free
ssh homelab
```

**Add to `~/.ssh/config` for permanent convenience:**
```
Host homelab
    HostName 192.168.x.x
    User dawreen
    IdentityFile ~/.ssh/id_ed25519
    AddKeysToAgent yes
    UseKeychain yes     # macOS: stores passphrase in Keychain
```

The container uses the same mechanism — `docker-compose.yml` mounts `/root/.ssh:/root/.ssh:ro` so `git push git@github.com:...` inside the container uses your SSH keys.

---

## Key Go Concepts Summary

| Concept | Explanation |
|---------|-------------|
| `package` | Every file belongs to a package. Same folder = same package. |
| `import` | Bring in other packages (stdlib, third-party, your own) |
| `struct` | Bundle of named fields — like a class without inheritance |
| `func f() (T, error)` | Multiple return values. Always check the error. |
| `:=` | Short variable declaration. Go infers the type. |
| `*T` | Pointer to type T. Holds a memory address, not a value. |
| `&x` | Take the address of x. Returns a pointer. |
| `*p` | Dereference pointer p. Go to that address and get the value. |
| `chan T` | Channel — typed pipe between goroutines |
| `go func()` | Start a goroutine (lightweight concurrent thread) |
| `defer f()` | Call f when the surrounding function returns |
| `context.Context` | Carries cancellation/deadline signals through call chains |
| `[]T` | Slice — dynamic array of type T |
| `...T` | Variadic — function accepts zero or more values of type T |
| `internal/` | Package only importable within the same module |
| Uppercase | Public — visible to other packages |
| lowercase | Private — only visible within the same package |

---

## Deployment Architecture

```
Development (Mac)
    make run       → go run, local bot for testing
    make build     → compile binary

Production (Homelab via Portainer)
    GitHub repo ← push code
    Portainer   ← pulls from GitHub, builds image, runs container
                   secrets managed in Portainer UI
                   notes persisted in Docker named volume
```
