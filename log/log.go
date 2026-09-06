package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wellbou/wellya/config"
)

type Level int

const (
	LVL_PANIC Level = iota
	LVL_ERROR
	LVL_WARNING
	LVL_INFO
)

var lvlName = map[Level]string{
	LVL_PANIC:   "PANIC",
	LVL_ERROR:   "ERROR",
	LVL_WARNING: "WARN",
	LVL_INFO:    "INFO",
}

var (
	file *os.File
	mux  sync.Mutex
)

const maxLogSize = 5 << 20

func getLogLocation() (string, error) {
	tempDir := filepath.Join(os.TempDir(), config.DirName)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(tempDir, config.DirName+".log"), nil
}

func Location() string {
	if file == nil {
		return ""
	}

	return file.Name()
}

func Start() {
	mux.Lock()
	defer mux.Unlock()

	path, err := getLogLocation()
	if err != nil {
		return
	}

	file, _ = os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
}

func Stop() {
	mux.Lock()
	defer mux.Unlock()

	if file == nil {
		return
	}
	file.Close()
	file = nil
}

func Print(lvl Level, format string, args ...any) {
	mux.Lock()
	defer mux.Unlock()

	if file == nil {
		return
	}

	if st, err := file.Stat(); err == nil && st.Size() > maxLogSize {
		_ = file.Truncate(0)
		_, _ = file.Seek(0, 0)
	}

	t := time.Now()
	format = fmt.Sprintf("[ %s ][(%d) %02d:%02d:%02d] - ", lvlName[lvl], t.Day(), t.Hour(), t.Minute(), t.Second()) + format + "\n"
	fmt.Fprintf(file, format, args...)
	file.Sync()
}
