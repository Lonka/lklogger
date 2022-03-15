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
	newLogger(&Service, "Service")
	newLogger(&API, "API")
}

func newLogger(log *logging.LoggerBase, serviceName string) {
	log.Logger = logging.NewLogger("", zapcore.InfoLevel, 100, 3, 7, true, serviceName)
	log.ServiceName = serviceName
}
```

## Use it
```go
func main(){
    loggerService.Serve()
	loggerService.Service.Info("Try it")
}
```
