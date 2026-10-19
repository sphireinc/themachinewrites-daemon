package publication

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type secretPattern struct {
	name string
	expr *regexp.Regexp
}

var secretPatterns = []secretPattern{
	{name: "GitHub token", expr: regexp.MustCompile(`(?i)\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})\b`)},
	{name: "OpenAI key", expr: regexp.MustCompile(`\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{20,}\b`)},
	{name: "Cloudflare token", expr: regexp.MustCompile(`(?i)\b(?:CLOUDFLARE_API_TOKEN|CF_API_TOKEN|THEMACHINEWRITES_GIT_PUBLISH_TOKEN)\s*[:=]\s*["']?[A-Za-z0-9_./+=-]{20,}`)},
	{name: "private key", expr: regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`)},
	{name: "AWS access key", expr: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{name: "Slack token", expr: regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{20,}\b`)},
	{name: "generic bearer", expr: regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/-]{24,}=*\b`)},
}

func ScanRenderedPost(content []byte) error {
	text := string(content)
	for _, pattern := range secretPatterns {
		if pattern.expr.MatchString(text) {
			return fmt.Errorf("publication contains a possible %s; quarantine required", pattern.name)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "THEMACHINEWRITES_GIT_PUBLISH_TOKEN=") || strings.HasPrefix(trimmed, "THEMACHINEWRITES_RESEARCH_TOKEN=") {
			return errors.New("publication contains a possible daemon credential; quarantine required")
		}
	}
	return nil
}

func QuarantineRenderedPost(root, postID string, content []byte, detectedAt time.Time) (string, error) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(postID) == "" {
		return "", errors.New("quarantine root and post ID are required")
	}
	if !validULID(postID) {
		return "", errors.New("quarantine post ID must be a ULID")
	}
	if detectedAt.IsZero() {
		return "", errors.New("quarantine timestamp is required")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", errors.New("quarantine root is invalid")
	}
	if err := os.MkdirAll(absoluteRoot, 0o700); err != nil {
		return "", fmt.Errorf("create quarantine root: %w", err)
	}
	info, err := os.Lstat(absoluteRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("quarantine root must be a real directory")
	}
	directory := filepath.Join(absoluteRoot, postID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create quarantine directory: %w", err)
	}
	path := filepath.Join(directory, detectedAt.UTC().Format("20060102T150405.000000000Z")+".md")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create quarantine file: %w", err)
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return "", fmt.Errorf("write quarantine file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return "", fmt.Errorf("sync quarantine file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close quarantine file: %w", err)
	}
	return path, nil
}
