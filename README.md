# lklogger

## Create loggerService.go first
```go
package loggerService

import (
	logging "github.com/Lonka/lklogger"
	"go.uber.org/zap/zapcore"
)

var (
	Service logging.LoggerBase
	API     logging.LoggerBase
)

func Serve() {
	overrideCfg := &config.LogConfig{
		Level:      "info",
		Format:     "json",
	}

	logging.Init("config/config.yml", overrideCfg)
	// Use the default configuration if the configPath is empty
	// logging.Init("", nil)
	Service = logging.NewServiceLogger("Service", false)
}
```

## Use it
```go
func main(){
    loggerService.Serve()
	loggerService.Service.Info("Try it")
}
```

## Configuration file
```yaml
# ----------------------------------------------------
# Logging Configuration
# ----------------------------------------------------
logger:
  # Log output level. Possible values:
  #   debug - detailed logs for development
  #   info  - general runtime information
  #   warn  - warnings and errors only
  #   error - errors only
  level: warn

  # Log format. Options:
  #   text - human-readable format
  #   json - structured format for log collectors or analysis tools
  format: text

  # Directory where log files will be stored.
  # Each service will generate its own log file.
  output_dir: ./log

  # Maximum size (in megabytes) of a single log file
  # before it gets rotated.
  max_size_mb: 10

  # Maximum number of old log files to keep.
  # Older files will be deleted when the limit is exceeded.
  max_backup_files: 7

  # Maximum number of days to retain old log files.
  # Files older than this will be removed automatically.
  max_age_days: 7

  # Whether to compress old log files (using gzip).
  # Set to true to save disk space.
  compress: true
```
