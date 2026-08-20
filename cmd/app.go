package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/djarbz/GoMPEG/application"
	"github.com/djarbz/GoMPEG/cmd/flag"
	"github.com/djarbz/GoMPEG/config"
	"golang.org/x/exp/slog"
)

func app(cmd *cobra.Command, _ []string) error {
	appConfig, err := config.NewServerConfig(cmd)
	if err != nil {
		return fmt.Errorf("compile configuration: %w", err)
	}

	debug, err := cmd.Flags().GetBool(flag.Debug)
	if err != nil {
		return fmt.Errorf(flag.ErrorFormat, flag.Debug, err)
	}
	if debug {
		viper.Debug()
		binaryName := strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]))
		fmt.Printf("Environment Variables should be prefixed with %s_\n", strings.ToUpper(binaryName))
		fmt.Printf("OS Environment:\n%s\n", os.Environ())
		fmt.Printf("Application Config: \n%v\n", appConfig)
	}

	// Reset logger to default upon end of func.
	defer func(l *slog.Logger) { slog.SetDefault(l) }(slog.Default())

	logger, err := NewMultiSlogger(cmd, appConfig.AppName)
	if err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}

	// slog.SetDefault(logger)

	// logger.With(slog.Any("Config", appConfig)).Debug("Logging configured!")

	if err := appConfig.CreateDirectories(); err != nil {
		return fmt.Errorf("creating application directories: %w", err)
	}

	_, err = config.OpenDatabase(logger.With("DBFile", appConfig.DBFile), appConfig.DBFile)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	// appConfig.DB = db

	if debug {
		logger.With(slog.Any("Config", appConfig)).Info("Application configured!")
	}

	return application.Process(logger, appConfig)
}
