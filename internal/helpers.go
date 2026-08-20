package internal

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/djarbz/GoMPEG/config"
	"github.com/gabriel-vasile/mimetype"
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

	return files, nil
}

func deleteEmptyDirectories(filePath string) error {
	return filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
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
}

// moveClean attempts atomic rename first, falling back to copy+delete across mounts
func moveClean(log logger, file *fileInfo, destinationDirectory string) error {
	if err := os.MkdirAll(destinationDirectory, 0755); err != nil {
		return fmt.Errorf("ensure destination directory: %w", err)
	}

	targetPath := filepath.Join(destinationDirectory, filepath.Base(file.path))

	// Fast Path: Atomic filesystem move
	if err := os.Rename(file.path, targetPath); err == nil {
		_ = deleteEmptyDirectories(file.rootPath)
		return nil
	}

	// Slow Path: Cross-device copy
	inputFile, err := os.Open(file.path)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer deferClose(log, inputFile)

	outputFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("open destination file: %w", err)
	}
	defer deferClose(log, outputFile)

	if _, err = io.Copy(outputFile, inputFile); err != nil {
		return fmt.Errorf("writing to output file failed: %w", err)
	}

	if err = os.Remove(file.path); err != nil {
		return fmt.Errorf("removing original file: %w", err)
	}

	_ = deleteEmptyDirectories(file.rootPath)
	return nil
}

func checkMime(log logger, conf *config.ServerConfig, file *fileInfo) (bool, error) {
	mime, err := mimetype.DetectFile(file.path)
	if err != nil {
		_ = moveClean(log, file, conf.VideoFailedDir)
		return false, fmt.Errorf("detect mime type: %w", err)
	}

	mimeStr := strings.ToLower(mime.String())
	if strings.HasPrefix(mimeStr, "video") || strings.HasPrefix(mimeStr, "audio") {
		return true, nil
	}

	log.Info("File is not a valid media type", slog.String("Mime", mime.String()))
	_ = moveClean(log, file, conf.VideoSkipDir)
	return false, nil
}

func deferClose(log logger, f *os.File) {
	if err := f.Close(); err != nil {
		log.Error("Could not close file.", slog.String("error", err.Error()), slog.String("Closer", f.Name()))
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

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func checkHash(log logger, conf *config.ServerConfig, file *fileInfo) error {
	hash := ""
	newHash, err := calcHash(log, file.path)
	if err != nil {
		return fmt.Errorf("calculate initial filehash: %w", err)
	}

	for i := 1; newHash != hash; i++ {
		if i > 10 {
			return fmt.Errorf("file still in motion after 10 checks")
		}
		time.Sleep(5 * time.Second)
		hash = newHash
		newHash, err = calcHash(log, file.path)
		if err != nil {
			return fmt.Errorf("calculate new filehash: %w", err)
		}
	}

	file.hash = newHash
	found, err := config.LookupHash(log, conf.DBFile, file.hash)
	if err != nil {
		return fmt.Errorf("lookup hash: %w", err)
	}

	if found {
		log.Info("Media file is a duplicate")
		if err := moveClean(log, file, conf.VideoSkipDir); err != nil {
			return fmt.Errorf("move duplicate file to skip dir: %w", err)
		}
		return fmt.Errorf("file is duplicate")
	}

	return nil
}

func verifyMedia(log logger, conf *config.ServerConfig, file *fileInfo, checkDuplicates bool) error {
	isValid, err := checkMime(log, conf, file)
	if err != nil || !isValid {
		return fmt.Errorf("invalid mime type")
	}

	if _, err := os.Stat(file.path); err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	if checkDuplicates {
		if err := checkHash(log, conf, file); err != nil {
			return fmt.Errorf("hash check: %w", err)
		}
	}

	return nil
}

func cleanDirectory(dir string) error {
	dirRead, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open directory: %w", err)
	}
	defer func() { _ = dirRead.Close() }()

	dirContents, err := dirRead.Readdir(0)
	if err != nil {
		return fmt.Errorf("list directory contents: %w", err)
	}

	for _, dirContent := range dirContents {
		fullPath := filepath.Join(dir, dirContent.Name())
		if err := os.RemoveAll(fullPath); err != nil {
			return fmt.Errorf("delete path [%s]: %w", fullPath, err)
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

func execute(appLog *slog.Logger, conf *config.ServerConfig, sourceFile *fileInfo, tempFile *fileInfo, outDir string, checkDuplicates bool, ffmpegArgs ...string) error {
	logBuffer := bytes.Buffer{}
	log := &execLogger{
		appLog:  appLog,
		execLog: slog.New(slog.NewJSONHandler(&logBuffer, nil)).With(slog.String("File", sourceFile.basename)),
	}

	if err := verifyMedia(log, conf, sourceFile, checkDuplicates); err != nil {
		return fmt.Errorf("verify source: %w", err)
	}

	if err := cleanDirectory(conf.TempDir); err != nil {
		return fmt.Errorf("clean temp directory: %w", err)
	}

	startTime := time.Now()
	log.Info("Starting FFmpeg execution")

	cmd := exec.Command("ffmpeg", ffmpegArgs...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("FFREPORT=file=%s:level=32", tempFile.reportFile))

	out, err := cmd.CombinedOutput()
	if err != nil {
		appLog.Error("FFmpeg failed to process file",
			slog.String("error", err.Error()),
			slog.String("output", string(out)),
		)

		_ = moveClean(log, sourceFile, conf.VideoFailedDir)
		_ = cleanDirectory(conf.TempDir)
		return fmt.Errorf("run FFmpeg: %w", err)
	}

	log.Info("FFmpeg complete", slog.Duration("duration", time.Since(startTime)))

	tempFile.info, err = os.Stat(tempFile.path)
	if err != nil {
		return fmt.Errorf("stat temp output file: %w", err)
	}

	if err := moveClean(log, tempFile, outDir); err != nil {
		return fmt.Errorf("move output file: %w", err)
	}
	if err := moveClean(log, sourceFile, conf.VideoCompleteDir); err != nil {
		return fmt.Errorf("move source file: %w", err)
	}
	_ = cleanDirectory(conf.TempDir)

	return nil
}
