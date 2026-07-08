package logger

import (
	"log/slog"
	"os"
)

const defaultAppLogFile = "logs/app.log"

func New(env string) *slog.Logger {
	file, err := OpenJSONLogFile(defaultAppLogFile)
	if err != nil {
		return slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	if env == "production" {
		return slog.New(slog.NewJSONHandler(file, nil))
	}

	return slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
