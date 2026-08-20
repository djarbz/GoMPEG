package application

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/djarbz/GoMPEG/config"
	"golang.org/x/exp/slog"
)

type logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type fileInfo struct {
	path       string
	info       os.FileInfo
	rootPath   string
	basename   string
	hash       string
	reportFile string
}

func dirFiles(rootDir string, exclude ...string) ([]*fileInfo, error) {
	var files []*fileInfo

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("[%s]: %w", path, err)
		}

		if info.IsDir() {
			return nil
		}

		for _, i := range exclude {
			if i == info.Name() {
				return nil
			}
		}
		files = append(files, &fileInfo{
			path:     path,
			info:     info,
			rootPath: rootDir,
			basename: info.Name()[:len(info.Name())-len(filepath.Ext(info.Name()))],
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	// Shuffle files, so we don't get stuck in an infinite loop
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })

	return files, nil
}

func deleteEmptyDirectories(filePath string) error {
	err := filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("[%s]: %w", path, err)
		}

		if info.IsDir() {
			if path == filePath {
				return nil
			}

			files, err := os.ReadDir(path)
			if err != nil {
				return fmt.Errorf("populate files in directory: %w", err)
			}

			if len(files) != 0 {
				return nil
			}

			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove empty directory: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("checking for empty directories: %w", err)
	}

	return nil
}

func moveClean(log logger, file *fileInfo, destinationDirectory string) error {
	if _, err := os.Stat(destinationDirectory); os.IsNotExist(err) {
		if err := os.Mkdir(destinationDirectory, 0755); err != nil {
			return fmt.Errorf("create destination directory: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("detect destination directory: %w", err)
	}

	inputFile, err := os.Open(file.path)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer deferClose(log, inputFile)

	outputFile, err := os.Create(filepath.Join(destinationDirectory, file.info.Name()))
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}

	defer deferClose(log, outputFile)

	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		return fmt.Errorf("writing to output file failed: %w", err)
	}

	// The copy was successful, so now delete the original file
	err = os.Remove(file.path)
	if err != nil {
		return fmt.Errorf("removing original file: %w", err)
	}

	if err := deleteEmptyDirectories(file.rootPath); err != nil {
		return fmt.Errorf("delete empty directories: %w", err)
	}

	return nil
}

func checkMime(log logger, conf *config.ServerConfig, file *fileInfo) (bool, error) {
	mime, err := mimetype.DetectFile(file.path)
	if err != nil {
		if err := moveClean(log, file, conf.VideoFailedDir); err != nil {
			return false, fmt.Errorf("move unknown file to failed dir: %w", err)
		}

		return false, fmt.Errorf("detect mime type: %w", err)
	}

	if strings.HasPrefix(strings.ToLower(mime.String()), "video") {
		return true, nil
	}

	log.Info("File does not have a video Mime type.", slog.Any("Mime", mime))

	if err := moveClean(log, file, conf.VideoSkipDir); err != nil {
		return false, fmt.Errorf("move file to skipped dir: %w", err)
	}

	return false, nil
}

func deferClose(log logger, f *os.File) {
	if err := f.Close(); err != nil {
		log.Error("Could not close file.", err, slog.String("Closer", f.Name()))
	}
}

func calcHash(log logger, filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}

	defer deferClose(log, f)

	hash := md5.New()

	if _, err := io.Copy(hash, f); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)[:16]), nil
}

func checkHash(log logger, conf *config.ServerConfig, file *fileInfo) error {
	hash := ""
	newHash, err := calcHash(log, file.path)
	if err != nil {
		return fmt.Errorf("calculate initial filehash: %w", err)
	}

	for i := 1; newHash != hash; i++ {
		if i > 10 {
			return fmt.Errorf("file still in motion, returning for next file")
		}

		if i == 1 {
			log.Info("Checking if file is in motion...", slog.Int("Check", i))
		} else {
			log.Info("File still in motion, waiting 15 seconds...", slog.Int("Check", i))
		}
		time.Sleep(15 * time.Second)

		hash = newHash

		newHash, err = calcHash(log, file.path)
		if err != nil {
			return fmt.Errorf("calculate new filehash: %w", err)
		}
	}

	file.hash = newHash

	log.Info("File at rest, checking database for hash.", slog.String("Hash", file.hash))
	found, err := config.LookupHash(log, conf.DBFile, file.hash)
	if err != nil {
		return fmt.Errorf("lookup hash: %w", err)
	}

	if found {
		log.Info("Video file is a duplicate.")

		if err := moveClean(log, file, conf.VideoSkipDir); err != nil {
			return fmt.Errorf("move file to skipped dir: %w", err)
		}
	}

	return nil
}

func verifyVideo(log logger, conf *config.ServerConfig, file *fileInfo) error {
	// Skip file if it is not a video mime type.
	isVideo, err := checkMime(log, conf, file)
	if err != nil {
		return fmt.Errorf("checking Mime type: %w", err)
	}
	if !isVideo {
		return fmt.Errorf("not a video file")
	}

	// Verify the file still exists before processing it.
	if _, err := os.Stat(file.path); os.IsNotExist(err) {
		return fmt.Errorf("file disappeared")
	} else if err != nil {
		return fmt.Errorf("determine if file still exists: %w", err)
	}

	// Check the Hash against previously converted files.
	if err := checkHash(log, conf, file); err != nil {
		return fmt.Errorf("hash check: %w", err)
	}

	return nil
}

func cleanDirectory(dir string) error {
	// if err := os.RemoveAll(dir); err != nil {
	// 	return fmt.Errorf("delete directory: %w", err)
	// }
	//
	// if err := os.MkdirAll(dir, os.ModePerm); err != nil {
	// 	return fmt.Errorf("recreate directory: %w", err)
	// }

	// Open the directory and read all its files.
	dirRead, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open directory: %w", err)
	}

	dirContents, err := dirRead.Readdir(0)
	if err != nil {
		return fmt.Errorf("list directory contents: %w", err)
	}

	for _, dirContent := range dirContents {
		fullPath := filepath.Join(dir, dirContent.Name())
		if err := os.RemoveAll(fullPath); err != nil {
			return fmt.Errorf("delete directory: %w", err)
		}
	}

	return nil
}

type execLogger struct {
	appLog  *slog.Logger
	execLog *slog.Logger
}

func (el *execLogger) Debug(msg string, args ...interface{}) {
	el.appLog.Debug(msg, args...)
	el.execLog.Debug(msg, args...)
}

func (el *execLogger) Info(msg string, args ...interface{}) {
	el.appLog.Info(msg, args...)
	el.execLog.Info(msg, args...)
}

func (el *execLogger) Error(msg string, args ...interface{}) {
	el.appLog.Error(msg, args...)
	el.execLog.Error(msg, args...)
}

func (el *execLogger) Write(dir string, name os.FileInfo, data []byte) error {
	logPath := filepath.Join(dir, name.Name()+".log")
	logFile, err := os.OpenFile(
		logPath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o644, //nolint:gomnd
	)
	if err != nil {
		return fmt.Errorf("open log file [%s] Error: %w", logPath, err)
	}

	defer deferClose(el.appLog, logFile)

	if _, err := logFile.Write(data); err != nil {
		return fmt.Errorf("save log file [%s] Error: %w", logPath, err)
	}

	return nil
}

func execute(appLog *slog.Logger, conf *config.ServerConfig, sourceFile *fileInfo, tempFile *fileInfo, outDir string,
	ffmpegArgs ...string) error {
	logBuffer := bytes.Buffer{}
	log := &execLogger{
		appLog:  appLog,
		execLog: slog.New(slog.NewJSONHandler(&logBuffer)).With(slog.String("File", sourceFile.info.Name())),
	}

	log.Info("Verifying source")

	if err := verifyVideo(log, conf, sourceFile); err != nil {
		return fmt.Errorf("verify source: %w", err)
	}

	log.Info("All checks passed!")

	if err := cleanDirectory(conf.TempDir); err != nil {
		return fmt.Errorf("clean temp directory: %w", err)
	}

	startTime := time.Now()
	log.Info("Starting ffmpeg", slog.Any("StartTimestamp", startTime))

	log.Debug("Command Arguments", slog.String("FFMpeg_args", strings.Join(ffmpegArgs, " ")))
	cmd := exec.Command("ffmpeg", ffmpegArgs...)
	log.Debug("Command Line", slog.String("FFMpeg_command", cmd.String()))

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("FFMpeg failed to process file.", err, slog.String("Output", string(out)))

		if err := moveClean(log, sourceFile, conf.VideoFailedDir); err != nil {
			log.Error("Failed to move source file to failed dir.", slog.String("Error", err.Error()))
		}

		if err := log.Write(conf.VideoFailedDir, sourceFile.info, logBuffer.Bytes()); err != nil {
			appLog.Error("Failed to write exec file log.", slog.String("Error", err.Error()))
		}

		if err := cleanDirectory(conf.TempDir); err != nil {
			log.Error("Failed to clean temp dir.", slog.String("Error", err.Error()))
		}
		return fmt.Errorf("run FFMpeg: %w", err)
	}

	elapsedTime := time.Since(startTime)
	log.Info("FFmpeg complete!", slog.Duration("Duration", elapsedTime))
	log.execLog.Info("Command Ouptut", slog.String("Output", string(out)))

	tempFile.info, err = os.Stat(tempFile.path)
	if err != nil {
		return fmt.Errorf("stat temp file: %w", err)
	}

	if err := moveClean(log, tempFile, outDir); err != nil {
		return fmt.Errorf("move temp file to output dir: %w", err)
	}

	if err := moveClean(log, sourceFile, conf.VideoCompleteDir); err != nil {
		return fmt.Errorf("move source file to complete dir: %w", err)
	}

	if err := log.Write(conf.VideoCompleteDir, sourceFile.info, logBuffer.Bytes()); err != nil {
		appLog.Error("Failed to write exec file log.", slog.String("Error", err.Error()))
	}

	if err := cleanDirectory(conf.TempDir); err != nil {
		return fmt.Errorf("clean temp directory: %w", err)
	}

	return nil
}
