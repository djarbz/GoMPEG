package internal

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/djarbz/GoMPEG/config"
	"github.com/fsnotify/fsnotify"
)

func Process(log *slog.Logger, conf *config.ServerConfig) error {
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := watchAndProcess(log.With("Processor", "Audio"), conf, conf.AudioWatchDir, processAudio); err != nil {
			log.Error("Audio pipeline failed", slog.String("error", err.Error()))
		}
	}()

	go func() {
		defer wg.Done()
		if err := watchAndProcess(log.With("Processor", "Video"), conf, conf.VideoWatchDir, processVideo); err != nil {
			log.Error("Video pipeline failed", slog.String("error", err.Error()))
		}
	}()

	wg.Wait()
	return nil
}

type processorFunc func(log *slog.Logger, conf *config.ServerConfig, sourceFile *fileInfo) error

func watchAndProcess(log *slog.Logger, conf *config.ServerConfig, watchDir string, processor processorFunc) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.Add(watchDir); err != nil {
		return fmt.Errorf("watch directory: %w", err)
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	processSweep(log, conf, watchDir, processor)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				time.Sleep(2 * time.Second)
				processSweep(log, conf, watchDir, processor)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Error("Watcher error", slog.String("error", err.Error()))
		case <-ticker.C:
			processSweep(log, conf, watchDir, processor)
		}
	}
}

func processSweep(log *slog.Logger, conf *config.ServerConfig, sourceDir string, processor processorFunc) {
	sourceFiles, err := dirFiles(sourceDir)
	if err != nil {
		log.Error("Failed to populate source files", slog.String("error", err.Error()))
		return
	}

	for i, file := range sourceFiles {
		processLog := log.With(slog.String("File", file.basename))
		processLog.Info("Processing media item", slog.Int("Remaining", len(sourceFiles)-i))

		if err := processor(processLog, conf, file); err != nil {
			processLog.Error("Failed to process media item", slog.String("error", err.Error()))
		}
	}
}
