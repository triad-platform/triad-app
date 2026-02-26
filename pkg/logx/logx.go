package logx

import (
	"os"

	"github.com/rs/zerolog"
)

func New() zerolog.Logger {
	// Keep it simple; can enrich later (service name, env, etc.)
	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
