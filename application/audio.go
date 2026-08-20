package application

import (
	"fmt"
	"path/filepath"

	"github.com/djarbz/GoMPEG/config"
	"golang.org/x/exp/slog"
)

// func processAudio(log *slog.Logger, conf *config.ServerConfig, file *fileInfo) error {
// 	log.Info("Verifying source")
//
// 	if err := verifyVideo(log, conf, file); err != nil {
// 		return fmt.Errorf("verify source: %w", err)
// 	}
//
// 	log.Info("All checks passed!")
//
// 	if err := cleanDirectory(conf.TempDir); err != nil {
// 		return fmt.Errorf("clean temp directory: %w", err)
// 	}
//
// 	tempFile := &fileInfo{
// 		path:     filepath.Join(conf.TempDir, file.basename+".mp3"),
// 		info:     nil,
// 		rootPath: conf.TempDir,
// 		basename: file.basename,
// 	}
//
// 	log.Info("Starting extraction...")
// 	startTime := time.Now()
//
// 	args := []string{
// 		"-i", file.path,
// 		"-vn",
// 		"-acodec", "libmp3lame",
// 		tempFile.path,
// 		// "2>", tempLog.path,
// 	}
//
// 	log.With(slog.String("FFMpeg Args", strings.Join(args, " "))).Debug("Command Arguments")
// 	cmd := exec.Command("ffmpeg", args...)
// 	log.With(slog.String("FFMpeg command", cmd.String())).Debug("Command Line")
//
// 	out, err := cmd.CombinedOutput()
// 	if err != nil {
// 		log.With(
// 			slog.String("Output", string(out)),
// 			slog.Any("Error", err),
// 		).Error("FFMpeg failed to extract audio.")
//
// 		if err := moveClean(log, file, conf.VideoFailedDir); err != nil {
// 			log.With(slog.Any("Error", err)).Error("Failed to move video to failed dir.")
// 		}
// 		if err := cleanDirectory(conf.TempDir); err != nil {
// 			log.With(slog.Any("Error", err)).Error("Failed to clean temp dir.")
// 		}
// 		return fmt.Errorf("run FFMpeg: %w", err)
// 	}
//
// 	elapsedTime := time.Since(startTime)
// 	log.With(slog.String("Duration", elapsedTime.String())).Info("Conversion complete!")
//
// 	tempFile.info, err = os.Stat(tempFile.path)
// 	if err != nil {
// 		return fmt.Errorf("stat new MP3 file: %w", err)
// 	}
//
// 	if err := moveClean(log, tempFile, conf.AudioOutputDir); err != nil {
// 		return fmt.Errorf("move extracted audio to output dir: %w", err)
// 	}
//
// 	if err := moveClean(log, file, conf.VideoCompleteDir); err != nil {
// 		return fmt.Errorf("move source video to complete dir: %w", err)
// 	}
//
// 	if err := cleanDirectory(conf.TempDir); err != nil {
// 		return fmt.Errorf("clean temp directory: %w", err)
// 	}
//
// 	return nil
// }

func processAudio(log *slog.Logger, conf *config.ServerConfig, sourceFile *fileInfo) error {
	tempFile := &fileInfo{
		path:       filepath.Join(conf.TempDir, sourceFile.basename+".m4a"),
		info:       nil,
		rootPath:   conf.TempDir,
		basename:   sourceFile.basename,
		reportFile: filepath.Join(conf.TempDir, sourceFile.basename+".ffmpeg.log"),
	}

	args := []string{
		"-hide_banner", // Suppress printing banner.
		"-nostats",     // Print encoding progress/statistics. It is on by default, to explicitly disable it you need to specify -nostats.
		"-i", sourceFile.path,
		fmt.Sprintf("-report:file=%s:level=32", tempFile.reportFile),
		"-vn",
		tempFile.path,
	}

	if err := execute(log, conf, sourceFile, tempFile, conf.AudioOutputDir, args...); err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	return nil
}
