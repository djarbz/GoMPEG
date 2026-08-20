package application

import (
	"fmt"
	"time"

	"github.com/djarbz/GoMPEG/config"
	"golang.org/x/exp/slog"
)

func Process(log *slog.Logger, conf *config.ServerConfig) error {
	for {
		if err := process(log.With(slog.String("Processor", "Audio")), conf, conf.AudioWatchDir, processAudio); err != nil {
			return fmt.Errorf("process Audio: %w", err)
		}

		if err := process(log.With(slog.String("Processor", "Video")), conf, conf.VideoWatchDir,
			processVideo); err != nil {
			return fmt.Errorf("process Video: %w", err)
		}

		time.Sleep(30 * time.Second)
	}

}

type processorFunc func(log *slog.Logger, conf *config.ServerConfig, sourceFile *fileInfo) error

func process(log *slog.Logger, conf *config.ServerConfig, sourceDir string, processor processorFunc) error {
	sourceFiles, err := dirFiles(sourceDir)
	if err != nil {
		return fmt.Errorf("populating source files: %w", err)
	}

	for i, file := range sourceFiles {
		processLog := log.With(slog.String("File", file.basename))
		processLog.Info("Processing...", slog.Int("Remaining", len(sourceFiles)-i))

		if err := processor(processLog, conf, file); err != nil {
			processLog.Error("Failed to process.", err)
		}
	}

	return nil
}
