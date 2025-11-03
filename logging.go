package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig defines the log configuration structure with mapstructure tags for Viper mapping.
type LogConfig struct {
	Level          string `mapstructure:"level"`            // Log level (debug, info, warn, error)
	Format         string `mapstructure:"format"`           // Format (text, json) - mainly affects encoder selection
	OutputDir      string `mapstructure:"output_dir"`       // Log file output directory
	MaxSizeMB      int    `mapstructure:"max_size_mb"`      // Log file rotation size (MB)
	MaxBackupFiles int    `mapstructure:"max_backup_files"` // Maximum number of backup files
	MaxAgeDays     int    `mapstructure:"max_age_days"`     // Maximum retention days
	Compress       bool   `mapstructure:"compress"`         // Whether to compress backup files
}

// LoggerBase is the log instance structure exposed for external use.
type LoggerBase struct {
	Logger      *zap.Logger
	WriteToAll  bool
	ServiceName string
}

var (
	// all is the global logger, initialized during Init.
	all *zap.Logger
	// currentConfig stores the final loaded configuration.
	currentConfig LogConfig
)

// stringToZapLevel converts a string log level to zapcore.Level.
func stringToZapLevel(s string) zapcore.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// loadConfig loads the 'logger' section from config.yml.
func loadConfig(configPath string) LogConfig {
	v := viper.New()

	// 1. Configure Viper to read the 'logger' section from config.yml
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 2. Read the configuration file
	if err := v.ReadInConfig(); err != nil {
		// Check if it’s a “file not found” error
		_, isViperNotFound := err.(viper.ConfigFileNotFoundError)
		isOSNotFound := os.IsNotExist(err)

		if !isViperNotFound && !isOSNotFound {
			// If it's another error (e.g. invalid format), treat as fatal
			log.Printf("Fatal: Failed to parse logging config file '%s': %v. Using default settings instead.", configPath, err)
		} else {
			log.Printf("Warning: Logging config file '%s' not found. Using default logging settings.", configPath)
		}
	}

	// 3. Set default values (lowest priority)
	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.format", "text")
	v.SetDefault("logger.output_dir", "./log")
	v.SetDefault("logger.max_size_mb", 10)
	v.SetDefault("logger.max_backup_files", 7)
	v.SetDefault("logger.max_age_days", 7)
	v.SetDefault("logger.compress", false)

	// Environment variable settings
	v.SetEnvPrefix("LK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	type Config struct {
		Logger LogConfig `mapstructure:"logger"`
	}

	// 4. Map only the 'logger' section
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		log.Fatalf("Fatal: Failed to unmarshal 'logger' section: %v", err)
	}
	return cfg.Logger
}

// Init initializes the global logging system and accepts an optional override configuration.
func Init(configPath string, overrideCfg *LogConfig) {
	// 1. Load configuration (from file or defaults)
	currentConfig = loadConfig(configPath)

	// 2. Apply override configuration if provided
	if overrideCfg != nil {
		if overrideCfg.Level != "" {
			currentConfig.Level = overrideCfg.Level
		}
		if overrideCfg.Format != "" {
			currentConfig.Format = overrideCfg.Format
		}
		if overrideCfg.OutputDir != "" {
			currentConfig.OutputDir = overrideCfg.OutputDir
		}
		if overrideCfg.MaxSizeMB != 0 {
			currentConfig.MaxSizeMB = overrideCfg.MaxSizeMB
		}
		if overrideCfg.MaxBackupFiles != 0 {
			currentConfig.MaxBackupFiles = overrideCfg.MaxBackupFiles
		}
		if overrideCfg.MaxAgeDays != 0 {
			currentConfig.MaxAgeDays = overrideCfg.MaxAgeDays
		}
		// Only override Compress when explicitly set to false
		if !overrideCfg.Compress {
			currentConfig.Compress = overrideCfg.Compress
		}
	}

	fmt.Printf("Loaded config: %+v\n", currentConfig)
	// 3. Initialize the global logger (All Logger) using the final configuration
	all = NewLoggerFromConfig(currentConfig, "All")
}

// NewLoggerFromConfig creates a zap.Logger instance using the given LogConfig.
func NewLoggerFromConfig(cfg LogConfig, serviceName string) *zap.Logger {
	core := newCoreFromConfig(cfg, serviceName)
	if serviceName == "All" {
		// Global "All" logger does not include a service_name field
		return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	}
	// Other service loggers include a service_name field
	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1), zap.Fields(zap.String("service_name", serviceName)))
}

// NewServiceLogger creates a new LoggerBase instance for a specific service.
func NewServiceLogger(serviceName string, writeToAll bool) LoggerBase {
	return LoggerBase{
		Logger:      NewLoggerFromConfig(currentConfig, serviceName),
		WriteToAll:  writeToAll,
		ServiceName: serviceName,
	}
}

func newCoreFromConfig(cfg LogConfig, serviceName string) zapcore.Core {
	level := stringToZapLevel(cfg.Level)

	// Configure file rotation
	logPath := filepath.Join(cfg.OutputDir, serviceName+".log")
	serviceFileLogger := getLumberjack(logPath, cfg.MaxSizeMB, cfg.MaxBackupFiles, cfg.MaxAgeDays, cfg.Compress)

	atomicLevel := zap.NewAtomicLevelAt(level)

	// Choose encoders based on format
	var consoleEncoder zapcore.Encoder
	var fileEncoder zapcore.Encoder

	if strings.ToLower(cfg.Format) == "json" {
		// JSON format
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = customizeTimeEncoder
		consoleEncoder = zapcore.NewJSONEncoder(encoderConfig)
		fileEncoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		// Default to console format (text)
		consoleEncoderConfig := getEncoderConfig()
		consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // Colored level for console
		fileEncoderConfig := getEncoderConfig()
		fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // No color for file output

		consoleEncoder = zapcore.NewConsoleEncoder(consoleEncoderConfig)
		fileEncoder = zapcore.NewConsoleEncoder(fileEncoderConfig)
	}

	// File output core
	fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(&serviceFileLogger), atomicLevel)

	if serviceName == "All" {
		// Global "All" logger outputs only to file
		return fileCore
	} else {
		// Service loggers output to both console (stdout) and file (Lumberjack)
		consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), atomicLevel)
		return zapcore.NewTee(consoleCore, fileCore)
	}
}

// customizeTimeEncoder formats timestamps in logs.
func customizeTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

// getLumberjack returns a configured log rotation instance.
func getLumberjack(filePath string, maxSize int, maxBackups int, maxAge int, compress bool) lumberjack.Logger {
	return lumberjack.Logger{
		Filename:   filePath,   // Log file path
		MaxSize:    maxSize,    // Max size per file (MB)
		MaxBackups: maxBackups, // Max number of backup files
		MaxAge:     maxAge,     // Max retention days
		Compress:   compress,   // Whether to compress old logs
	}
}

// getEncoderConfig returns the default encoder configuration.
func getEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     "\r\n",
		EncodeTime:     customizeTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}
}

// --- External logging interface (keeps consistency with original LoggerBase methods) ---

func (log LoggerBase) Debug(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	log.Logger.Debug(msg, fields...)
	if log.WriteToAll {
		fields = append([]zapcore.Field{zap.String("service_name", log.ServiceName)}, fields...)
		all.Debug(msg, fields...)
	}
}

func (log LoggerBase) Info(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	log.Logger.Info(msg, fields...)
	if log.WriteToAll {
		fields = append([]zapcore.Field{zap.String("service_name", log.ServiceName)}, fields...)
		all.Info(msg, fields...)
	}
}

func (log LoggerBase) Warn(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	log.Logger.Warn(msg, fields...)
	if log.WriteToAll {
		fields = append([]zapcore.Field{zap.String("service_name", log.ServiceName)}, fields...)
		all.Warn(msg, fields...)
	}
}

func (log LoggerBase) Error(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	log.Logger.Error(msg, fields...)
	if log.WriteToAll {
		fields = append([]zapcore.Field{zap.String("service_name", log.ServiceName)}, fields...)
		all.Error(msg, fields...)
	}
}

func (log LoggerBase) Fatal(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	if log.WriteToAll {
		tempFields := append([]zapcore.Field{zap.String("service_name", log.ServiceName), zap.String("FATAL", "Exit")}, fields...)
		all.DPanic(msg, tempFields...)
	}
	log.Logger.Fatal(msg, fields...)
}

func GetField(key, value string) zapcore.Field {
	return zap.String(key, value)
}
