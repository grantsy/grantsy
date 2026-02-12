package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/phsym/console-slog"
)

func Setup(level, format, env string) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		panic(err)
	}

	opts := &slog.HandlerOptions{
		Level:       lvl,
		AddSource:   true,
		ReplaceAttr: replaceAttr,
	}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		if env == "dev" {
			handler = console.NewHandler(os.Stdout, &console.HandlerOptions{
				Level:     lvl,
				AddSource: true,
			})
		} else {
			handler = slog.NewTextHandler(os.Stdout, opts)
		}
	default:
		panic("invalid log format: " + format)
	}

	slog.SetDefault(slog.New(handler))
}

func replaceAttr(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey:
		if t, ok := a.Value.Any().(time.Time); ok {
			a.Value = slog.StringValue(t.UTC().Format(time.RFC3339))
		}
	case slog.SourceKey:
		if src, ok := a.Value.Any().(*slog.Source); ok {
			src.File = shortenSourcePath(src.File)
		}
	}
	return a
}

func shortenSourcePath(path string) string {
	prefixes := []string{
		"/go/pkg/mod/",
		"/build/",
		"/grantsy/",
		"/projects/grantsy/",
	}

	for _, prefix := range prefixes {
		if _, after, ok := strings.Cut(path, prefix); ok {
			shortened := after
			if atIdx := strings.Index(shortened, "@"); atIdx != -1 {
				if slashIdx := strings.Index(shortened[atIdx:], "/"); slashIdx != -1 {
					shortened = shortened[:atIdx] + shortened[atIdx+slashIdx:]
				}
			}
			return shortened
		}
	}

	return filepath.Base(path)
}
