package gitpub

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type GitRunner interface {
	Run(context.Context, string, []string, []string, []byte) result
}

type result struct {
	stdout string
	stderr string
	err    error
}

type commandRunner struct{}

func (commandRunner) Run(ctx context.Context, executable string, args, env []string, stdin []byte) result {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return result{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

type RepositoryConfig struct {
	Root           string
	Branch         string
	Remote         string
	ExpectedRemote string
}

type Repository struct {
	Root   string
	Branch string
	Remote string
	Runner GitRunner
}

func OpenRepository(ctx context.Context, config RepositoryConfig, runner GitRunner) (*Repository, error) {
	if !filepath.IsAbs(config.Root) {
		return nil, errors.New("public repository root must be absolute")
	}
	if config.Branch == "" || config.Remote == "" || config.ExpectedRemote == "" {
		return nil, errors.New("branch, remote and expected remote are required")
	}
	if runner == nil {
		runner = commandRunner{}
	}
	repo := &Repository{Root: filepath.Clean(config.Root), Branch: config.Branch, Remote: config.Remote, Runner: runner}
	top, err := repo.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	top = strings.TrimSpace(top)
	if !filepath.IsAbs(top) || filepath.Clean(top) != filepath.Clean(config.Root) {
		return nil, errors.New("configured root is outside public worktree")
	}
	branch, err := repo.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || strings.TrimSpace(branch) != config.Branch {
		return nil, fmt.Errorf("unexpected branch %q", strings.TrimSpace(branch))
	}
	remote, err := repo.run(ctx, "remote", "get-url", config.Remote)
	if err != nil || strings.TrimSpace(remote) != config.ExpectedRemote {
		return nil, fmt.Errorf("unexpected remote URL")
	}
	return repo, nil
}

func (r *Repository) SafePath(relative string) (string, error) {
	if filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", errors.New("path must be relative slash path")
	}
	clean := filepath.ToSlash(filepath.Clean(relative))
	if clean != relative || clean == "." || strings.HasPrefix(clean, "../") {
		return "", errors.New("path escapes repository")
	}
	allowed := strings.HasPrefix(clean, "posts/") || strings.HasPrefix(clean, "feed/") || strings.HasPrefix(clean, "schemas/")
	if !allowed {
		return "", errors.New("path outside publication allowlist")
	}
	if strings.HasSuffix(clean, ".env") || strings.Contains(clean, "..") {
		return "", errors.New("unsafe path")
	}
	if strings.HasPrefix(clean, "posts/") {
		if err := validatePostPath(clean); err != nil {
			return "", err
		}
	}
	root, err := filepath.Abs(r.Root)
	if err != nil {
		return "", errors.New("repository root is invalid")
	}
	candidate := filepath.Join(root, filepath.FromSlash(clean))
	relativeToRoot, err := filepath.Rel(root, candidate)
	if err != nil || relativeToRoot == "." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes repository")
	}
	if err := rejectSymlinkAncestors(root, relativeToRoot); err != nil {
		return "", err
	}
	return candidate, nil
}

func validatePostPath(path string) error {
	parts := strings.Split(path, "/")
	if len(parts) != 5 || parts[0] != "posts" || len(parts[1]) != 4 || len(parts[2]) != 2 || len(parts[3]) != 2 || len(parts[4]) != 29 || parts[4] != parts[4][:26]+".md" {
		return errors.New("post path must match posts/YYYY/MM/DD/<ULID>.md")
	}
	year, month, day := parts[1], parts[2], parts[3]
	if year[0] == '0' || month[0] == '0' && month[1] == '0' || day[0] == '0' && day[1] == '0' {
		return errors.New("post path contains invalid UTC date")
	}
	for _, value := range []string{year, month, day} {
		for _, character := range value {
			if character < '0' || character > '9' {
				return errors.New("post path contains invalid UTC date")
			}
		}
	}
	if parts[4][0] < '0' || parts[4][0] > '7' {
		return errors.New("post path contains invalid ULID")
	}
	for _, character := range parts[4][:26] {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", character) {
			return errors.New("post path contains invalid ULID")
		}
	}
	if !validPostDate(year, month, day) {
		return errors.New("post path contains invalid UTC date")
	}
	return nil
}

func validPostDate(year, month, day string) bool {
	at, err := time.Parse("2006-01-02", year+"-"+month+"-"+day)
	return err == nil && at.Format("2006-01-02") == year+"-"+month+"-"+day
}

func rejectSymlinkAncestors(root, relative string) error {
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect publication path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("publication path contains a symlink")
		}
	}
	return nil
}

func (r *Repository) run(ctx context.Context, args ...string) (string, error) {
	full := append([]string{"-C", r.Root}, args...)
	out := r.Runner.Run(ctx, "git", full, []string{"PATH=/usr/bin:/bin", "HOME=/private/tmp/core", "GIT_CONFIG_NOSYSTEM=1"}, nil)
	if out.err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), out.err, strings.TrimSpace(out.stderr))
	}
	return out.stdout, nil
}
