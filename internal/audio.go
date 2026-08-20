package internal

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/djarbz/GoMPEG/config"
)

func processAudio(log *slog.Logger, conf *config.ServerConfig, sourceFile *fileInfo) error {
	tempFile := &fileInfo{
		path:       filepath.Join(conf.TempDir, sourceFile.basename+".m4a"),
		info:       nil,
		rootPath:   conf.TempDir,
		basename:   sourceFile.basename,
		reportFile: filepath.Join(conf.TempDir, sourceFile.basename+".ffmpeg.log"),
	}

	args := []string{
		"-hide_banner",
		"-nostats",
		"-y",
		"-i", sourceFile.path,
		"-vn",
		"-codec:a", "aac",
		"-ac", "2",
		"-b:a", "192k",
		tempFile.path,
	}

	// Audio extraction passes 'false' for duplicate hash checking
	if err := execute(log, conf, sourceFile, tempFile, conf.AudioOutputDir, false, args...); err != nil {
		return fmt.Errorf("extract audio: %w", err)
	}

	return nil
}
