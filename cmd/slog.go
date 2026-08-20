package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/djarbz/GoMPEG/cmd/flag"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

func levelMatcher(level string) (slog.Level, error) {
	if levelInt, err := strconv.Atoi(level); err == nil {
		return slog.Level(levelInt), nil
	}

	switch strings.ToUpper(level) {
	case "TRACE":
		return slog.Level(-8), nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "NOTICE":
		return slog.Level(2), nil
	case "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	case "EMERG":
		return slog.Level(12), nil
	}

	return slog.LevelInfo, fmt.Errorf("unknown Log Level: %s", level)
}

func NewMultiSlogger(cmd *cobra.Command, appName string) (*slog.Logger, error) {
	flagLevel, _ := cmd.Flags().GetString(flag.LogLevel)
	level, _ := levelMatcher(flagLevel)

	debug, _ := cmd.Flags().GetBool(flag.Debug)
	if debug {
		level = slog.LevelDebug
	}

	addSource, _ := cmd.Flags().GetBool(flag.LogSource)
	fullSource, _ := cmd.Flags().GetBool(flag.LogFullSource)

	// Auto-detect non-TTY stdout (e.g. journald/podman container) or flag
	disableTime, _ := cmd.Flags().GetBool(flag.LogTimestampDisable)
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		disableTime = true
	}

	opts := &slog.HandlerOptions{
		AddSource: addSource,
		Level:     level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if disableTime && a.Key == slog.TimeKey {
				return slog.Attr{} // Strip timestamp for journald
			}
			if !fullSource && a.Key == slog.SourceKey {
				a.Value = slog.StringValue(filepath.Base(a.Value.String()))
			}
			return a
		},
	}

	logWriter, err := slogSetupMultiWriter(cmd)
	if err != nil {
		return nil, fmt.Errorf("setup Slog MultiWriter: %w", err)
	}

	format, _ := cmd.Flags().GetString(flag.LogConsoleFormat)
	var handler slog.Handler
	if strings.ToUpper(format) == "JSON" {
		handler = slog.NewJSONHandler(logWriter, opts)
	} else {
		handler = slog.NewTextHandler(logWriter, opts)
	}

	return slog.New(handler.WithAttrs([]slog.Attr{
		slog.String("name", appName),
	})), nil
}

func slogSetupMultiWriter(cmd *cobra.Command) (io.Writer, error) {
	debug, _ := cmd.Flags().GetBool(flag.Debug)

	var writers []io.Writer

	console, err := consoleWriter(cmd, debug)
	if err != nil {
		return nil, fmt.Errorf("setup console writer: %w", err)
	}
	if console != nil {
		writers = append(writers, console)
	}

	file, err := fileWriter(cmd, debug)
	if err != nil {
		return nil, fmt.Errorf("setup file writer: %w", err)
	}
	if file != nil {
		writers = append(writers, file)
	}

	if len(writers) == 0 {
		return os.Stdout, nil
	}

	if len(writers) == 1 {
		return writers[0], nil
	}

	return io.MultiWriter(writers...), nil
}

type lineFlushWriter struct {
	w *os.File
}

func (l *lineFlushWriter) Write(p []byte) (n int, err error) {
	n, err = l.w.Write(p)
	_ = l.w.Sync() // Force immediate flush to OS/journald
	return n, err
}

func consoleWriter(cmd *cobra.Command, debug bool) (io.Writer, error) {
	disableConsole, _ := cmd.Flags().GetBool(flag.LogConsoleDisable)
	if disableConsole {
		return nil, nil
	}
	// Return the unbuffered flusher over stdout
	return &lineFlushWriter{w: os.Stdout}, nil
}

func fileWriter(cmd *cobra.Command, debug bool) (io.Writer, error) {
	enableFile, _ := cmd.Flags().GetBool(flag.LogFileEnable)
	if !enableFile {
		return nil, nil
	}

	filePath, _ := cmd.Flags().GetString(flag.LogFilePath)
	if filePath == "" {
		return nil, fmt.Errorf("empty %s not allowed", flag.LogFilePath)
	}

	rotateEnable, _ := cmd.Flags().GetBool(flag.LogFileRotateEnable)
	if rotateEnable {
		return fileRotator(cmd, filePath)
	}

	return fileStatic(filePath)
}

func fileRotator(cmd *cobra.Command, filePath string) (io.Writer, error) {
	size, _ := cmd.Flags().GetInt(flag.LogFileRotateSize)
	keep, _ := cmd.Flags().GetInt(flag.LogFileRotateKeep)
	age, _ := cmd.Flags().GetInt(flag.LogFileRotateAge)

	return &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    size,
		MaxBackups: keep,
		MaxAge:     age,
	}, nil
}

func fileStatic(filePath string) (io.Writer, error) {
	logFile, err := os.OpenFile(
		filePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return nil, fmt.Errorf("open log file [%s]: %w", filePath, err)
	}

	return logFile, nil
}
