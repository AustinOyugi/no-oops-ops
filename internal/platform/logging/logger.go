package logging

import "log/slog"

func New() *slog.Logger {
	return slog.Default()
}
