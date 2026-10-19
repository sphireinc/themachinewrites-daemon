package supervisor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Lease struct {
	path string
	file *os.File
}

func AcquireLease(directory, workerID string) (*Lease, error) {
	if workerID == "" {
		return nil, errors.New("worker ID required")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "publish.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("publishing lease held: %w", err)
	}
	if _, err := file.WriteString(workerID); err != nil {
		file.Close()
		os.Remove(path)
		return nil, err
	}
	return &Lease{path: path, file: file}, nil
}
func (l *Lease) Release() error {
	if l == nil {
		return nil
	}
	err := l.file.Close()
	removeErr := os.Remove(l.path)
	if err != nil {
		return err
	}
	return removeErr
}
