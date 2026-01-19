package tools

import (
	"log/slog"
	"os"
)

// Logger is the global structured logger instance.
var Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))
