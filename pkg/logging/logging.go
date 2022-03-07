package logging

import (
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// LoggerSpecEnvVar is the environment variable that controls what kind of logger we
	// want.  If the value is "file" we will only log to file.  If the value is "both", we
	// log to console and file.  If the value is "console" we log to console only.
	// "container"
	LoggerSpecEnvVar = "TEST_LOGGER"

	// LogDirEnvVar is the environment variable that controls which directory logging
	// will take place in.  If this is unset we do not log to file.make
	LogDirEnvVar = "TEST_LOG_DIR"

	// LogFileSizeEnvVar specifies max log file size in megabytes
	LogFileSizeEnvVar = "TEST_LOG_FILE_SIZE_MB"

	// LogFileMaxAgeEnvVar is the maximum number of days we will keep log files around.
	LogFileMaxAgeEnvVar = "TEST_LOG_FILE_MAX_AGE_DAYS"

	// maxDurationForTemporaryLogLevelChange is the maximum amount of time we allow a
	// temporary log change to last
	maxDurationForTemporaryLogLevelChange = 60 * time.Minute

	// defaultTemporaryLogLevelChangeDuration is the default duration we switch log levels
	// for if no time is given.
	defaultTemporaryLogLevelChangeDuration = 5 * time.Minute

	logFileName = "test.log"
)

var (
	logger          *zap.Logger
	atomicLogLevel  = zap.NewAtomicLevel() // defaults to info
	defaultLogLevel = zapcore.InfoLevel
	lg              *zap.SugaredLogger
)

func init() {
	var core zapcore.Core

	// Choose between different logging configurations
	switch os.Getenv(LoggerSpecEnvVar) {
	// the "file" configuration means the logger will only log to files
	case "file":
		core = zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), getLogFileWriter(), atomicLogLevel)

	// the "both" configuration means the logger will log to console and files,
	// however, it will use a more human readable format for the console.
	case "both":
		core = zapcore.NewTee(
			zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), getLogFileWriter(), atomicLogLevel),
			zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()), zapcore.AddSync(os.Stderr), atomicLogLevel),
		)

	// "console" means the logger logs to console only.
	case "console":
		core = zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()), zapcore.AddSync(os.Stderr), atomicLogLevel)

	// "container" is a setting that logs JSON on stderr
	case "container":
		core = zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(os.Stderr), atomicLogLevel)

	// console logging with human readable format is default
	default:
		core = zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()), zapcore.AddSync(os.Stderr), atomicLogLevel)
	}

	logger = zap.New(core, zap.AddCaller())

	zap.RedirectStdLog(logger)
	zap.ReplaceGlobals(logger)

	lg = logger.Sugar()
}

func getLogFileWriter() zapcore.WriteSyncer {
	logFileSizeMB := int64(0)
	if os.Getenv(LogFileSizeEnvVar) != "" {
		size, err := strconv.ParseInt(os.Getenv(LogFileSizeEnvVar), 10, 64)
		if err == nil {
			logFileSizeMB = size
		}
	}

	// Figure out how long to keep log files
	maxAge := time.Duration(0)
	if os.Getenv(LogFileMaxAgeEnvVar) != "" {
		days, err := strconv.ParseInt(os.Getenv(LogFileMaxAgeEnvVar), 10, 32)
		if err == nil {
			maxAge = time.Duration(days) * time.Hour * 24
		}
	}

	return zapcore.AddSync(NewFileWriter(FileWriterConfig{
		LogDirName:          GetLogDir(),
		LogFileName:         logFileName,
		Compress:            true,
		MaxTimeTimeToKeep:   maxAge,
		MaxLogFileSizeBytes: logFileSizeMB * 1024 * 1024,
	}))
}
