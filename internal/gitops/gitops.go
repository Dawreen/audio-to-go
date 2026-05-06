package gitops

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// InitRepo sets up the git repository in the notes directory.
func InitRepo(remoteURL, userEmail, userName, notesDir string) error {
	// Set global git configs
	if err := runIn("/", "git", "config", "--global", "user.email", userEmail); err != nil {
		return fmt.Errorf("git config email: %w", err)
	}
	if err := runIn("/", "git", "config", "--global", "user.name", userName); err != nil {
		return fmt.Errorf("git config name: %w", err)
	}
	if err := runIn("/", "git", "config", "--global", "--add", "safe.directory", "/app"); err != nil {
		log.Printf("git config safe.directory: %v", err)
	}

	// Initialize git repo in the notes volume on first start.
	gitDir := filepath.Join(notesDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if err := runIn("/", "git", "config", "--global", "init.defaultBranch", "main"); err != nil {
			return fmt.Errorf("git config defaultBranch: %w", err)
		}

		if err := os.MkdirAll(notesDir, 0o755); err != nil {
			return fmt.Errorf("creating notes dir: %w", err)
		}

		if err := runIn(notesDir, "git", "init"); err != nil {
			return fmt.Errorf("git init: %w", err)
		}
		if err := runIn(notesDir, "git", "remote", "add", "origin", remoteURL); err != nil {
			return fmt.Errorf("git remote add: %w", err)
		}

		// Try to fetch and reset to the remote main branch to avoid conflicts
		if err := runIn(notesDir, "git", "fetch", "origin", "main"); err == nil {
			_ = runIn(notesDir, "git", "reset", "--hard", "origin/main")
		}
	}

	return nil
}

// Push stages all changes in the notes dir, commits, and pushes to the remote.
func Push(notesDir, remoteURL string) error {
	if err := runIn(notesDir, "git", "add", "."); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	msg := "note: " + time.Now().Format("2006-01-02 15:04")
	if err := runIn(notesDir, "git", "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Pull remote changes to avoid push rejection if there are remote updates.
	// We ignore the error because it might fail if the remote is completely empty.
	_ = runIn(notesDir, "git", "pull", "--rebase", remoteURL, "main")

	if err := runIn(notesDir, "git", "push", remoteURL, "HEAD:main", "--force-with-lease"); err != nil {
		// First push needs to set upstream; fall back to plain push.
		if err2 := runIn(notesDir, "git", "push", "-u", remoteURL, "HEAD:main"); err2 != nil {
			return fmt.Errorf("git push: %w", err2)
		}
	}

	return nil
}

func runIn(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
