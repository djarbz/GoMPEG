package config

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/djarbz/GoMPEG/cmd/flag"
)

func NewServerConfig(cmd *cobra.Command) (*ServerConfig, error) {
	srvCfg := new(ServerConfig)

	appName, err := cmd.Flags().GetString(flag.AppName)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.AppName, err)
	}

	srvCfg.AppName = appName

	videoWatchDir, err := cmd.Flags().GetString(flag.VideoWatchDir)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.VideoWatchDir, err)
	}

	srvCfg.VideoWatchDir = videoWatchDir

	videoOutputDir, err := cmd.Flags().GetString(flag.VideoOutputDir)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.VideoOutputDir, err)
	}

	srvCfg.VideoOutputDir = videoOutputDir

	videoSkipDir, err := cmd.Flags().GetString(flag.VideoSkipDir)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.VideoSkipDir, err)
	}

	srvCfg.VideoSkipDir = videoSkipDir

	videoFailedDir, err := cmd.Flags().GetString(flag.VideoFailedDir)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.VideoFailedDir, err)
	}

	srvCfg.VideoFailedDir = videoFailedDir

	videoCompleteDir, err := cmd.Flags().GetString(flag.VideoCompleteDir)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.VideoCompleteDir, err)
	}

	srvCfg.VideoCompleteDir = videoCompleteDir

	audioWatchDir, err := cmd.Flags().GetString(flag.AudioWatchDir)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.AudioWatchDir, err)
	}

	srvCfg.AudioWatchDir = audioWatchDir

	audioOutputDir, err := cmd.Flags().GetString(flag.AudioOutputDir)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.AudioOutputDir, err)
	}

	srvCfg.AudioOutputDir = audioOutputDir

	tempDir, err := cmd.Flags().GetString(flag.TempDir)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.TempDir, err)
	}

	srvCfg.TempDir = tempDir

	dbFile, err := cmd.Flags().GetString(flag.DBFile)
	if err != nil {
		return nil, fmt.Errorf(flag.ErrorFormat, flag.DBFile, err)
	}

	srvCfg.DBFile = dbFile

	return srvCfg, nil
}

type ServerConfig struct {
	AppName          string
	VideoWatchDir    string
	VideoOutputDir   string
	VideoSkipDir     string
	VideoFailedDir   string
	VideoCompleteDir string
	AudioWatchDir    string
	AudioOutputDir   string
	TempDir          string
	DBFile           string
	DB               *sql.DB
}

func (sc *ServerConfig) createDirectory(dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("creating directory [%s]: %w", dir, err)
	}
	return nil
}

func (sc *ServerConfig) CreateDirectories() error {
	if err := sc.createDirectory(sc.VideoWatchDir); err != nil {
		return err
	}

	if err := sc.createDirectory(sc.VideoOutputDir); err != nil {
		return err
	}

	if err := sc.createDirectory(sc.VideoSkipDir); err != nil {
		return err
	}

	if err := sc.createDirectory(sc.VideoFailedDir); err != nil {
		return err
	}

	if err := sc.createDirectory(sc.VideoCompleteDir); err != nil {
		return err
	}

	if err := sc.createDirectory(sc.AudioWatchDir); err != nil {
		return err
	}

	if err := sc.createDirectory(sc.AudioOutputDir); err != nil {
		return err
	}

	if err := sc.createDirectory(sc.TempDir); err != nil {
		return err
	}

	return nil
}
