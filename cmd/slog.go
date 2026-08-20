package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/djarbz/GoMPEG/cmd/flag"
	"gitlab.com/djarbz/golang/slogmw"
	"golang.org/x/exp/slog"
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
	flagLevel, err := cmd.Flags().GetString(flag.LogLevel)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogLevel, err)
	}
	level, err := levelMatcher(flagLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	debug, err := cmd.Flags().GetBool(flag.Debug)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.Debug, err)
	}
	if debug {
		level = slog.LevelDebug
	}

	addSource, err := cmd.Flags().GetBool(flag.LogSource)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogSource, err)
	}

	fullSource, err := cmd.Flags().GetBool(flag.LogFullSource)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogFullSource, err)
	}

	handler := slog.HandlerOptions{
		AddSource: addSource,
		Level:     level,
	}

	if !fullSource {
		handler.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			// Remove the directory from the source's filename.
			if a.Key == slog.SourceKey {
				a.Value = slog.StringValue(filepath.Base(a.Value.String()))
			}
			return a
		}
	}

	logWriter, err := slogSetupMultiWriter(cmd)
	if err != nil {
		return nil, fmt.Errorf("setup Slog MultiWriter: %w", err)
	}

	return slog.New(
		handler.NewJSONHandler(logWriter).
			WithAttrs([]slog.Attr{
				slog.String("name", appName),
			}),
	), nil
}

func slogSetupMultiWriter(cmd *cobra.Command) (io.Writer, error) {
	debug, err := cmd.Flags().GetBool(flag.Debug)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.Debug, err)
	}
	if debug {
		slogmw.VerboseErrors = true
	}

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

	if debug {
		fmt.Printf("Logging with %d writers\n", len(writers))
	}

	if len(writers) == 1 {
		if debug {
			fmt.Printf("Returning a single writer...\n")
		}
		return writers[0], nil
	}

	return io.MultiWriter(writers...), nil
}

func consoleWriter(cmd *cobra.Command, debug bool) (io.Writer, error) {
	disableConsole, err := cmd.Flags().GetBool(flag.LogConsoleDisable)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogConsoleDisable, err)
	}

	if disableConsole {
		if debug {
			fmt.Printf("Console logging disabled\n")
		}
		return nil, nil
	}

	format, err := cmd.Flags().GetString(flag.LogConsoleFormat)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogConsoleFormat, err)
	}

	if debug {
		fmt.Printf("Console logging type: %s\n", format)
	}

	return outputFormatWriter(format, os.Stdout), nil
}

func fileWriter(cmd *cobra.Command, debug bool) (io.Writer, error) {
	enableFile, err := cmd.Flags().GetBool(flag.LogFileEnable)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogFileEnable, err)
	}
	if !enableFile {
		if debug {
			fmt.Printf("File logging disabled\n")
		}
		return nil, nil
	}

	filePath, err := cmd.Flags().GetString(flag.LogFilePath)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogFilePath, err)
	}
	if filePath == "" {
		return nil, fmt.Errorf("empty %s not allowed", flag.LogFilePath)
	}

	format, err := cmd.Flags().GetString(flag.LogFileFormat)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogFileFormat, err)
	}

	if debug {
		fmt.Printf("Logging file path: %s\n", filePath)
		fmt.Printf("File logging type: %s\n", format)
	}

	rotateEnable, err := cmd.Flags().GetBool(flag.LogFileRotateEnable)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogFileRotateEnable, err)
	}
	if rotateEnable {
		logFile, err := fileRotator(cmd, debug, filePath, format)
		if err != nil {
			return nil, fmt.Errorf("setup log rotation: %w", err)
		}
		if logFile != nil {
			return logFile, nil
		}
	}

	logFile, err := fileStatic(debug, filePath, format)
	if err != nil {
		return nil, fmt.Errorf("setup static log: %w", err)
	}
	if logFile != nil {
		return logFile, nil
	}

	return nil, fmt.Errorf("unreachable code")
}

func fileRotator(cmd *cobra.Command, debug bool, filePath string, format string) (io.Writer, error) {
	size, err := cmd.Flags().GetInt(flag.LogFileRotateSize)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogFileRotateSize, err)
	}

	keep, err := cmd.Flags().GetInt(flag.LogFileRotateKeep)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogFileRotateKeep, err)
	}

	age, err := cmd.Flags().GetInt(flag.LogFileRotateAge)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.LogFileRotateAge, err)
	}

	rotator := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    size, // megabytes
		MaxBackups: keep, // files
		MaxAge:     age,  // days
	}

	if debug {
		fmt.Printf("Rotate Log file enabled\n")
		fmt.Printf("Rotate Log file %dMB\n", size)
		fmt.Printf("Rotate Log file keep %d files\n", keep)
		fmt.Printf("Rotate Log file %d days\n", age)
	}

	return outputFormatWriter(format, rotator), nil
}

func fileStatic(debug bool, filePath string, format string) (io.Writer, error) {
	logFile, err := os.OpenFile(
		filePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o644, //nolint:gomnd
	)
	if err != nil {
		return nil, fmt.Errorf("open log file [%s] Error: %w", filePath, err)
	}

	if err := logFile.Sync(); err != nil {
		return nil, fmt.Errorf("sync log file [%s] Error: %w", filePath, err)
	}

	if debug {
		fmt.Printf("Standard Log: %s\n", logFile.Name())
	}

	return outputFormatWriter(format, logFile), nil
}

const (
	FormatText = "TEXT"
	FormatJSON = "JSON"
)

func outputFormatWriter(format string, writer io.Writer) io.Writer {
	switch format {
	case FormatJSON:
		// log.Println("Utilizing JSON")
		return slogmw.NewJSON(writer)
	case FormatText:
		// log.Println("Utilizing LOGFMT")
		return slogmw.NewLOGFMT(writer)
	default:
		// log.Println("Defaulting to LOGFMT")
		return slogmw.NewLOGFMT(writer)
	}
}
