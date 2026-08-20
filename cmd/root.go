package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/djarbz/GoMPEG/cmd/flag"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gompeg",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	RunE: app,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	if _, err := os.Stat(".env"); err == nil {
		// log.Printf("Loading .env\n")

		if err := godotenv.Load(".env"); err != nil {
			log.Fatalf("Failed to load DOTENV. Err: %v", err)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		// log.Printf("DOTENV not found, continuing...\n")
	} else {
		// Schrodinger: file may or may not exist. See err for details.
		// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
		log.Printf("Failed to determine if DOTENV file exists: %v", err)
	}

	// log.Printf("OS Environment:\n%s\n", os.Environ())

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (searches in /etc, $HOME/.config, and the local dir)")

	rootCmd.Flags().Bool(flag.Debug, false, "Enable debugging of application startup.")
	rootCmd.Flags().StringP(flag.AppName, "a", "gompeg", "Application name for logging purposes.")
	// rootCmd.Flags().Bool(flag.Development, false, "Enable Development mode.")
	rootCmd.Flags().String(flag.VideoWatchDir, "/convert/_transcode/new/video",
		"Directory to watch for new video files.")
	rootCmd.Flags().String(flag.VideoOutputDir, "/convert/converted",
		"Directory to save the converted file after a successful conversion.")
	rootCmd.Flags().String(flag.VideoSkipDir, "/convert/_transcode/processed/skipped",
		"Directory to move the source file if matching a previous video.")
	rootCmd.Flags().String(flag.VideoFailedDir, "/convert/_transcode/processed/failed",
		"Directory to move the source file after a failed conversion.")
	rootCmd.Flags().String(flag.VideoCompleteDir, "/convert/_transcode/processed/completed",
		"Directory to move the source file after a successful conversion.")
	rootCmd.Flags().String(flag.AudioWatchDir, "/convert/_transcode/new/audio",
		"Directory to watch for new video files.")
	rootCmd.Flags().String(flag.AudioOutputDir, "/convert/audio/_NEW",
		"Directory to move the source file after a successful conversion.")
	rootCmd.Flags().String(flag.TempDir, "/convert/temp",
		"Directory to copy the source file to start conversion.")
	rootCmd.Flags().String(flag.DBFile, "/convert/_transcode/processed/processed.db",
		"Filepath to save the hash of successful conversions.")
	rootCmd.Flags().Bool(flag.LogConsoleDisable, false, "Disable logging to STDERR.")
	rootCmd.Flags().String(flag.LogLevel, "INFO", "Set the log level to the string or int of TRACE=-8, "+
		"DEBUG=-4, INFO=0, NOTICE=2, WARNING=4, ERROR=8, EMERG=12 or any numerical value for your purposes.")
	rootCmd.Flags().String(flag.LogConsoleFormat, "TEXT", "Console logging format of either TEXT(LOGFMT) or JSON.")
	rootCmd.Flags().Bool(flag.LogSource, false, "Enable logging of the source file.")
	rootCmd.Flags().Bool(flag.LogFullSource, false, "Print the full path of the source file, "+
		"requires "+flag.LogSource+".")
	rootCmd.Flags().Bool(flag.LogFileEnable, false, "Enable logging to a specified file.")
	rootCmd.Flags().String(flag.LogFilePath, "/convert/_transcode/gompeg.log", "File to write the logs to.")
	rootCmd.Flags().String(flag.LogFileFormat, "TEXT", "Console logging format of either TEXT(LOGFMT) or JSON.")
	rootCmd.Flags().Bool(flag.LogFileRotateEnable, true, "Enable log rotation.")
	rootCmd.Flags().Int(flag.LogFileRotateSize, 1, "Max size in Megabytes of log file before rotation.") //nolint:gomnd
	rootCmd.Flags().Int(flag.LogFileRotateKeep, 3, "Number of rotated logs to keep.")                    //nolint:gomnd
	rootCmd.Flags().Int(flag.LogFileRotateAge, 7, "Max age (days) of a log file.")                       //nolint:gomnd
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	binaryName := strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]))

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for Config in /etc
		viper.AddConfigPath(filepath.Join("/etc", binaryName))

		// Find userConfigDir directory.
		userConfigDir, err := os.UserConfigDir()
		// cobra.CheckErr(err)
		if err != nil {
			log.Printf("Failed to check User's config directory: %e", err)
			userConfigDir = ""
		}
		if userConfigDir != "" {
			// Search in the user's configuration directory ( $XDG_CONFIG_HOME or $HOME/.config ).
			viper.AddConfigPath(filepath.Join(userConfigDir, binaryName))
		}

		// Search for config in the local dir.
		viper.AddConfigPath(".")

		// Search for config named config.yaml
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvPrefix(binaryName)
	// log.Printf("Environment Variables should be prefixed with %s_", strings.ToUpper(binaryName))
	viper.SetEnvKeyReplacer(strings.NewReplacer("_", "-"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		_, _ = fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	// viper.Debug()
}
