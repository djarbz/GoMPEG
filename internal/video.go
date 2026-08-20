package internal

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/djarbz/GoMPEG/config"
)

func processVideo(log *slog.Logger, conf *config.ServerConfig, sourceFile *fileInfo) error {
	tempFile := &fileInfo{
		path:       filepath.Join(conf.TempDir, sourceFile.basename+".mp4"),
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
		"-codec:v", "libsvtav1",
		"-crf", "26",
		"-preset", "6",
		"-pix_fmt", "yuv420p10le",
		"-codec:a", "aac",
		"-ac", "2",
		"-b:a", "160k",
		"-movflags", "+faststart",
		tempFile.path,
	}

	// Video processing passes 'true' for duplicate hash checking
	if err := execute(log, conf, sourceFile, tempFile, conf.VideoOutputDir, true, args...); err != nil {
		return fmt.Errorf("convert video: %w", err)
	}

	if err := config.SaveHash(log, conf.DBFile, sourceFile.hash, sourceFile.info.Name()); err != nil {
		log.Error("Save hash to database.", slog.String("error", err.Error()))
	}

	return nil
}
