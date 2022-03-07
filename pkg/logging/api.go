package logging

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Get the logger
func Get() *zap.Logger {
	return logger
}

// SetLevel sets the log level
func SetLevel(level zapcore.Level) {
	atomicLogLevel.SetLevel(level)
}

// GetLevel returns the current log level
func GetLevel() zapcore.Level {
	return atomicLogLevel.Level()
}

// GetLogDir returns the directory we log to
func GetLogDir() string {
	if os.Getenv(LogDirEnvVar) != "" {
		return os.Getenv(LogDirEnvVar)
	}

	return defaultLogDirName
}

// SetLevelTemporarily sets the loglevel to `level` for `d` duration.
func SetLevelTemporarily(level zapcore.Level, d time.Duration) (time.Duration, error) {
	// special case:  if we are setting the loglevel to the defaultLogLevel, we don't have
	// to reset it.  We just return the maximum duration possible and no error.
	if level == defaultLogLevel {
		atomicLogLevel.SetLevel(defaultLogLevel)
		return 1<<63 - 1, nil
	}

	// cap duration at maxDurationForTemporaryLogLevelChange
	if d == 0 {
		d = defaultTemporaryLogLevelChangeDuration
	}
	if d > maxDurationForTemporaryLogLevelChange {
		d = maxDurationForTemporaryLogLevelChange
	}

	go func() {
		<-time.After(d)

		// There may not be a need to reset the log level
		if atomicLogLevel.Level() == defaultLogLevel {
			return
		}

		lg.Infow("resetting loglevel", "from", atomicLogLevel.Level(), "to", defaultLogLevel)
		atomicLogLevel.SetLevel(defaultLogLevel)
	}()

	atomicLogLevel.SetLevel(level)
	return d, nil
}
