package application

import (
	"fmt"
	"path/filepath"

	"github.com/djarbz/GoMPEG/config"
	"golang.org/x/exp/slog"
)

// func processVideo(log *slog.Logger, conf *config.ServerConfig, file *fileInfo) error {
// 	log.Info("Verifying video")
//
// 	if err := verifyVideo(log, conf, file); err != nil {
// 		return fmt.Errorf("verify video: %w", err)
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
// 	log.Info("Starting conversion...")
// 	startTime := time.Now()
//
// 	args := []string{
// 		"-i", file.path,
// 		"-f", "mp4",
// 		"-vcodec", "libx264",
// 		"-preset", "fast",
// 		"-profile:v", "main",
// 		"-acodec", "aac",
// 		"-movflags", "+faststart",
// 		"-hide_banner",
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
// 		).Error("FFMpeg failed to convert video.")
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
// 		return fmt.Errorf("stat new MP4 file: %w", err)
// 	}
//
// 	if err := moveClean(log, tempFile, conf.AudioOutputDir); err != nil {
// 		return fmt.Errorf("move converted video to output dir: %w", err)
// 	}
//
// 	if err := moveClean(log, file, conf.VideoCompleteDir); err != nil {
// 		return fmt.Errorf("move source video to complete dir: %w", err)
// 	}
//
// 	if err := config.SaveHash(log, conf.DBFile, file.hash, file.info.Name()); err != nil {
// 		log.With(slog.Any("Error", err)).Info("Saving hash to database.")
// 	}
//
// 	if err := cleanDirectory(conf.TempDir); err != nil {
// 		return fmt.Errorf("clean temp directory: %w", err)
// 	}
//
// 	return nil
// }

func processVideo(log *slog.Logger, conf *config.ServerConfig, sourceFile *fileInfo) error {
	tempFile := &fileInfo{
		path:       filepath.Join(conf.TempDir, sourceFile.basename+".mp4"),
		info:       nil,
		rootPath:   conf.TempDir,
		basename:   sourceFile.basename,
		reportFile: filepath.Join(conf.TempDir, sourceFile.basename+".ffmpeg.log"),
	}

	args := []string{
		"-hide_banner",        // Suppress printing banner.
		"-nostats",            // Print encoding progress/statistics. It is on by default, to explicitly disable it you need to specify -nostats.
		"-i", sourceFile.path, // input file name
		// "-sameq", // Use same video quality as source (implies VBR ).
		fmt.Sprintf("-report:file=%s:level=32", tempFile.reportFile),
		"-codec:a", "ac3", // Force audio codec to codec.
		// Use the "copy" special value to specify that the raw codec data must be copied as is.
		"-movflags", "+faststart",
		tempFile.path,
	}

	// H.264 Encoding
	// args = append(
	// 	args,
	// 	"-codec:v", "libx264", // Force video codec to codec.
	// 	Use the "copy" special value to tell that the raw codec data must be copied as is.
	// "-profile:v", "main",
	// "-preset", "fast",
	// 	)

	// AV1 Encoding
	// TODO: Enable support for libsvtav1
	args = append(
		args,
		"-codec:v", "libaom-av1",
		// The CRF value can be from 0 to 63.
		// Lower values mean better quality and greater file size.
		// 0 means lossless.
		// A CRF value of 23 yields a quality level corresponding to CRF 19 for x264, which would be considered visually lossless.
		"-crf", "23",
	)

	if err := execute(log, conf, sourceFile, tempFile, conf.VideoOutputDir, args...); err != nil {
		return fmt.Errorf("convert: %w", err)
	}

	if err := config.SaveHash(log, conf.DBFile, sourceFile.hash, sourceFile.info.Name()); err != nil {
		log.Error("Save hash to database.", err)
	}

	return nil
}
