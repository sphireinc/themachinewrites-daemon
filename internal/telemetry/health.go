package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxHealthSnapshotBytes = 16 * 1024

type Health struct {
	SchemaVersion     int        `json:"schema_version"`
	LastPoll          *time.Time `json:"last_poll"`
	LastModelCall     *time.Time `json:"last_model_call"`
	LastConfirmedPush *time.Time `json:"last_confirmed_push"`
	LastErrorKind     string     `json:"last_error_kind"`
	PausedReason      string     `json:"paused_reason"`
}

func EmptyHealth() Health { return Health{SchemaVersion: 1} }

func ReadHealth(path string) (Health, error) {
	if strings.TrimSpace(path) == "" {
		return Health{}, errors.New("health path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return Health{}, fmt.Errorf("read health snapshot: %w", err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxHealthSnapshotBytes+1))
	if err != nil {
		return Health{}, fmt.Errorf("read health snapshot: %w", err)
	}
	if len(content) > maxHealthSnapshotBytes {
		return Health{}, errors.New("health snapshot exceeds size limit")
	}
	var health Health
	if err := json.Unmarshal(content, &health); err != nil {
		return Health{}, fmt.Errorf("decode health snapshot: %w", err)
	}
	if health.SchemaVersion != 1 {
		return Health{}, errors.New("unsupported health snapshot schema")
	}
	if err := validateHealthLabels(health); err != nil {
		return Health{}, err
	}
	return health.Clone(), nil
}

func WriteHealth(path string, health Health) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("health path is required")
	}
	health = health.Clone()
	health.SchemaVersion = 1
	if err := validateHealthLabels(health); err != nil {
		return err
	}
	content, err := json.MarshalIndent(health, "", "  ")
	if err != nil {
		return fmt.Errorf("encode health snapshot: %w", err)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create health directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".health-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary health snapshot: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("set health permissions: %w", err)
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return fmt.Errorf("write health snapshot: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync health snapshot: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close health snapshot: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace health snapshot: %w", err)
	}
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open health directory: %w", err)
	}
	defer directoryHandle.Close()
	if err := directoryHandle.Sync(); err != nil {
		return fmt.Errorf("sync health directory: %w", err)
	}
	return nil
}

func validateHealthLabels(health Health) error {
	if !validHealthLabel(health.LastErrorKind, 64) {
		return errors.New("last_error_kind must be a bounded machine label")
	}
	if !validHealthLabel(health.PausedReason, 64) {
		return errors.New("paused_reason must be a bounded machine label")
	}
	return nil
}

func validHealthLabel(value string, maximum int) bool {
	if value == "" {
		return true
	}
	if len(value) > maximum {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' || character == '-') {
			return false
		}
	}
	return true
}

func (h Health) Clone() Health {
	h.LastPoll = cloneTime(h.LastPoll)
	h.LastModelCall = cloneTime(h.LastModelCall)
	h.LastConfirmedPush = cloneTime(h.LastConfirmedPush)
	return h
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
