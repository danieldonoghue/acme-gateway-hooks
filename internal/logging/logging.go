package logging

import (
	"log/slog"
	"os"
)

// Logger provides structured loggers split by output stream.
type Logger struct {
	Info  *slog.Logger
	Error *slog.Logger
}

func New(service string) Logger {
	info := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
	}).WithAttrs([]slog.Attr{slog.String("service", service)}))

	err := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: false,
	}).WithAttrs([]slog.Attr{slog.String("service", service)}))

	return Logger{Info: info, Error: err}
}
