package logging

import (
	"os"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LoggerBase struct {
	Logger      *zap.Logger
	WriteToAll  bool
	ServiceName string
}

var (
	all = NewLogger("", zapcore.InfoLevel, 10, 7, 7, true, "All")
	// InfoLevel = zapcore.InfoLevel
)

func getRunMode() string {
	cfg, err := ini.Load("setting.ini")
	if err != nil {
		// log.Fatalf("Fail to parse 'setting.ini':'%v'", err)
		return "release"
	}
	runMode := cfg.Section("").Key("RunMode").MustString("debug")
	if runMode == "" {
		runMode = "release"
	}
	return runMode
}

func getLogPathAndExt() (path, ext string) {
	cfg, err := ini.Load("setting.ini")
	if err != nil {
		// log.Fatalf("Fail to parse 'setting.ini':'%v'", err)
		return "./log/", "log"
	}
	path = cfg.Section("app").Key("LogSavePath").MustString("./log/")
	ext = cfg.Section("app").Key("LogFileExt").MustString("log")
	return
}

/*
  NewLogger Create Logger
  filePath log path
  level log level
  maxSize keep size per file, unit:M
  maxBackups maximum backup log files
  maxAge maximum keep days
  compress is compress
  serviceName service name
*/

func NewLogger(filePath string, level zapcore.Level, maxSize int, maxBackups int, maxAge int, compress bool, serviceName string) *zap.Logger {
	path, ext := getLogPathAndExt()
	realFilePath := filePath
	if filePath == "" {
		realFilePath = path + strings.ToLower(serviceName) + "." + ext
	}
	core := newCore(realFilePath, level, maxSize, maxBackups, maxAge, compress, serviceName)
	if serviceName == "All" {
		return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	} else {
		return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1), zap.Fields(zap.String("service name", serviceName)))
	}
}

func newCore(filePath string, level zapcore.Level, maxSize int, maxBackups int, maxAge int, compress bool, serviceName string) zapcore.Core {

	serviceFileLogger := getLumberjack(filePath, maxSize, maxBackups, maxAge, compress)

	atomicLevel := zap.NewAtomicLevel()
	if getRunMode() == "debug" {
		atomicLevel.SetLevel(zapcore.DebugLevel)
	} else {
		atomicLevel.SetLevel(level)
	}

	consoleEncoderConfig := getEncoderConfig()
	consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	fileEncoderConfig := getEncoderConfig()
	fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)
	fileEncoder := zapcore.NewConsoleEncoder(fileEncoderConfig)

	if serviceName == "All" {
		return zapcore.NewTee(
			zapcore.NewCore(fileEncoder, zapcore.AddSync(&serviceFileLogger), atomicLevel),
		)
	} else {
		return zapcore.NewTee(
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), atomicLevel),
			zapcore.NewCore(fileEncoder, zapcore.AddSync(&serviceFileLogger), atomicLevel),
		)
	}
}

func customizeTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05"))
}

func getLumberjack(filePath string, maxSize int, maxBackups int, maxAge int, compress bool) lumberjack.Logger {
	return lumberjack.Logger{
		Filename:   filePath,   // log path
		MaxSize:    maxSize,    // keep size per file, unit:M
		MaxBackups: maxBackups, // maximum backup log files
		MaxAge:     maxAge,     // maximum keep days
		Compress:   compress,   // is compress
	}
}

func getEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     "\r\n",                         //\n
		EncodeTime:     customizeTimeEncoder,           // ISO8601 UTC
		EncodeDuration: zapcore.SecondsDurationEncoder, //
		//EncodeCaller: zapcore.FullCallerEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
		EncodeName:   zapcore.FullNameEncoder,
	}
}

func (log LoggerBase) Debug(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	log.Logger.Debug(msg, fields...)
	if log.WriteToAll {
		fields = append([]zapcore.Field{zap.String("service name", log.ServiceName)}, fields...)
		all.Debug(msg, fields...)
	}
}

func (log LoggerBase) Info(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	log.Logger.Info(msg, fields...)
	if log.WriteToAll {
		fields = append([]zapcore.Field{zap.String("service name", log.ServiceName)}, fields...)
		all.Info(msg, fields...)
	}
}

func (log LoggerBase) Warn(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	log.Logger.Warn(msg, fields...)
	if log.WriteToAll {
		fields = append([]zapcore.Field{zap.String("service name", log.ServiceName)}, fields...)
		all.Warn(msg, fields...)
	}
}
func (log LoggerBase) Error(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	log.Logger.Error(msg, fields...)
	if log.WriteToAll {
		fields = append([]zapcore.Field{zap.String("service name", log.ServiceName)}, fields...)
		all.Error(msg, fields...)
	}
}

func (log LoggerBase) Fatal(msg string, fields ...zapcore.Field) {
	defer log.Logger.Sync()
	defer all.Sync()
	if log.WriteToAll {
		tempFields := append([]zapcore.Field{zap.String("service name", log.ServiceName), zap.String("FATAL", "Exit")}, fields...)
		all.DPanic(msg, tempFields...)
	}
	log.Logger.Fatal(msg, fields...)
}

func GetField(key, value string) zapcore.Field {
	return zap.String(key, value)
}
