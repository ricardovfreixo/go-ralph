package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	logFile *os.File
	enabled bool
)

func Init(workDir string) error {
	mu.Lock()
	defer mu.Unlock()

	ralphDir := filepath.Join(workDir, ".ralph")
	if err := os.MkdirAll(ralphDir, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(ralphDir, "ralph.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	logFile = f
	enabled = true

	write("INFO", "ralph", "Logging initialized", "path", logPath)
	return nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	enabled = false
}

func write(level, component, msg string, kvs ...interface{}) {
	if !enabled || logFile == nil {
		return
	}

	timestamp := time.Now().Format("15:04:05.000")
	line := fmt.Sprintf("[%s] %s %s: %s", timestamp, level, component, msg)

	for i := 0; i < len(kvs)-1; i += 2 {
		line += fmt.Sprintf(" %v=%v", kvs[i], kvs[i+1])
	}
	line += "\n"

	logFile.WriteString(line)
}

func Info(component, msg string, kvs ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	write("INFO", component, msg, kvs...)
}

func Debug(component, msg string, kvs ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	write("DEBUG", component, msg, kvs...)
}

func Error(component, msg string, kvs ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	write("ERROR", component, msg, kvs...)
}

func Warn(component, msg string, kvs ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	write("WARN", component, msg, kvs...)
}
