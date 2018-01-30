package logging

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Formatter is the struct used in the logging package.
type Formatter struct {
}

// Format builds the log desired log format.
func (c *Formatter) Format(entry *log.Entry) ([]byte, error) {
	return []byte(fmt.Sprintf("%s [%s] %s\n", entry.Time.Format("2006/01/02 15:04:05"), strings.ToUpper(entry.Level.String()), entry.Message)), nil
}

func init() {
	log.SetFormatter(&Formatter{})
}

// SetLevel sets the log level.
// Accepted levels are panic, fatal, error, warn, info and debug.
func SetLevel(level string) {
	lvl, err := log.ParseLevel(level)
	if err != nil {
		Fatal(fmt.Sprintf(`not a valid level: "%s"`, level))
	}
	log.SetLevel(lvl)
}

// Debug logs a message with severity DEBUG.
func Debug(format string, v ...interface{}) {
	// log.Debug(fmt.Sprintf(format, v...))
	log.Debugf(format, v...)
}

// Info logs a message with severity INFO.
func Info(format string, v ...interface{}) {
	log.Info(fmt.Sprintf(format, v...))
}

// Warning logs a message with severity WARNING.
func Warning(format string, v ...interface{}) {
	log.Warning(fmt.Sprintf(format, v...))
}

// Error logs a message with severity ERROR.
func Error(format string, v ...interface{}) {
	log.Error(fmt.Sprintf(format, v...))
}

// Fatal logs a message with severity ERROR which is then followed by a call
// to os.Exit().
func Fatal(format string, v ...interface{}) {
	log.Fatal(fmt.Sprintf(format, v...))
}

// IsDebug returns true if current level is debug
func IsDebug() bool {
	return log.GetLevel() == log.DebugLevel
}

// IsInfo returns true if current level is info
func IsInfo() bool {
	return log.GetLevel() == log.InfoLevel
}

// IsWarning returns true if current level is warn
func IsWarning() bool {
	return log.GetLevel() == log.WarnLevel
}

// IsError returns true if current level is error
func IsError() bool {
	return log.GetLevel() == log.ErrorLevel
}

// IsFatal returns true if current level is fatal
func IsFatal() bool {
	return log.GetLevel() == log.FatalLevel
}

// IsPanic returns true if current level is panic
func IsPanic() bool {
	return log.GetLevel() == log.PanicLevel
}
