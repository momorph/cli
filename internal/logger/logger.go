package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/momorph/cli/internal/config"
	"github.com/rs/zerolog"
)

var (
	// Log is the global logger instance
	Log zerolog.Logger
)

// Init initializes the logger with the specified configuration
func Init(debug bool) error {
	// Ensure logs directory exists
	if err := config.EnsureLogsDir(); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Set log level
	logLevel := zerolog.InfoLevel
	if debug {
		logLevel = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Create log file with date-based rotation
	logFile, err := getLogFile()
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Create multi-writer (file + console for debug mode)
	var writers []io.Writer
	writers = append(writers, logFile)

	if debug {
		// Add console output for debug mode with pretty formatting
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		}
		writers = append(writers, consoleWriter)
	}

	multi := io.MultiWriter(writers...)

	// Initialize logger
	Log = zerolog.New(multi).With().
		Timestamp().
		Str("app", "momorph-cli").
		Logger()

	Log.Debug().Msg("Logger initialized")
	return nil
}

// getLogFile returns the log file for the current date
func getLogFile() (*os.File, error) {
	logsDir := config.GetLogsDir()

	// Generate log filename with current date
	logFileName := fmt.Sprintf("momorph-%s.log", time.Now().Format("2006-01-02"))
	logFilePath := filepath.Join(logsDir, logFileName)

	// Open or create log file
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	// Clean up old log files (keep last 7 days)
	go cleanOldLogs(logsDir, 7)

	return logFile, nil
}

// cleanOldLogs removes log files older than the specified number of days
func cleanOldLogs(logsDir string, keepDays int) {
	files, err := os.ReadDir(logsDir)
	if err != nil {
		return
	}

	cutoffTime := time.Now().AddDate(0, 0, -keepDays)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if file matches log pattern
		if filepath.Ext(file.Name()) != ".log" {
			continue
		}

		filePath := filepath.Join(logsDir, file.Name())
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		// Remove if older than cutoff
		if fileInfo.ModTime().Before(cutoffTime) {
			os.Remove(filePath)
		}
	}
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	if len(args) == 0 {
		Log.Debug().Msg(format)
	} else {
		Log.Debug().Msgf(format, args...)
	}
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	if len(args) == 0 {
		Log.Info().Msg(format)
	} else {
		Log.Info().Msgf(format, args...)
	}
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	if len(args) == 0 {
		Log.Warn().Msg(format)
	} else {
		Log.Warn().Msgf(format, args...)
	}
}

// Error logs an error message
func Error(msg string, err error) {
	Log.Error().Err(err).Msg(msg)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	Log.Error().Msgf(format, args...)
}

// Fatal logs a fatal error and exits
func Fatal(msg string, err error) {
	Log.Fatal().Err(err).Msg(msg)
}

// Fatalf logs a formatted fatal error and exits
func Fatalf(format string, args ...interface{}) {
	Log.Fatal().Msgf(format, args...)
}
