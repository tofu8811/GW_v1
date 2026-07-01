package logger

import (
	"os"
	"path/filepath"
)

const defaultLogFile = "logs/gateway.jsonl"

func OpenJSONLogFile(path string) (*os.File, error) {
	if path == "" {
		path = defaultLogFile
	}

	resolvedPath := resolveProjectPath(path)
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return nil, err
	}

	return os.OpenFile(resolvedPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
}

func resolveProjectPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return filepath.Clean(path)
	}

	for dir := workingDir; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, path)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	return filepath.Join(workingDir, path)
}
