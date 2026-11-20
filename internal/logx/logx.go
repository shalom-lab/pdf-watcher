package logx

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	mu  sync.Mutex
	dir string
	day string
	f   *os.File
}

func New(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	l := &Logger{dir: dir}
	if err := l.rotate(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Logger) rotate() error {
	day := time.Now().Format("2006-01-02")
	if l.f != nil && l.day == day {
		return nil
	}
	if l.f != nil {
		_ = l.f.Close()
		l.f = nil
	}
	path := filepath.Join(l.dir, day+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	l.f = f
	l.day = day
	return nil
}

func (l *Logger) log(level, path, stage, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.rotate()
	line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\n",
		time.Now().Format("2006-01-02 15:04:05"),
		level, path, stage, msg)
	if l.f != nil {
		_, _ = l.f.WriteString(line)
	}
}

func (l *Logger) Info(path, stage, msg string)  { l.log("INFO", path, stage, msg) }
func (l *Logger) Warn(path, stage, msg string)  { l.log("WARN", path, stage, msg) }
func (l *Logger) Error(path, stage, msg string) { l.log("ERROR", path, stage, msg) }

func (l *Logger) Infof(path, stage, format string, args ...any) {
	l.Info(path, stage, fmt.Sprintf(format, args...))
}
func (l *Logger) Warnf(path, stage, format string, args ...any) {
	l.Warn(path, stage, fmt.Sprintf(format, args...))
}
func (l *Logger) Errorf(path, stage, format string, args ...any) {
	l.Error(path, stage, fmt.Sprintf(format, args...))
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f != nil {
		err := l.f.Close()
		l.f = nil
		return err
	}
	return nil
}
