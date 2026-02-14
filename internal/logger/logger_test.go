package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_DefaultVsFileOutput(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "test.log")

	fileLog, err := New(tmpfile, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fileLog.logger.Error("file message")
	fileLog.Sync()

	content, _ := os.ReadFile(tmpfile)
	if !strings.Contains(string(content), "file message") {
		t.Error("expected message in log file")
	}

	defaultLog, err := New("", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defaultLog.logger.Error("stderr message")
	defaultLog.Sync()

	content2, _ := os.ReadFile(tmpfile)
	if strings.Contains(string(content2), "stderr message") {
		t.Error("default logger should not write to file")
	}
}

func TestNew_InvalidPath(t *testing.T) {
	_, err := New("/invalid/path/that/does/not/exist/test.log", false, false)
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestNew_DebugMode(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "test.log")

	log, err := New(tmpfile, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer log.Sync()

	log.Debug("debug msg")
	log.Info("info msg")
	log.Warn("warn msg")
	log.Error("error msg")
	log.Sync()

	content, _ := os.ReadFile(tmpfile)
	output := string(content)

	// All levels should appear in debug mode
	if !strings.Contains(output, "debug msg") {
		t.Error("debug mode should log debug messages")
	}
	if !strings.Contains(output, "info msg") {
		t.Error("debug mode should log info messages")
	}
}

func TestNew_VerboseMode(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "test.log")

	log, err := New(tmpfile, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer log.Sync()

	log.Debug("debug msg")
	log.Info("info msg")
	log.Warn("warn msg")
	log.Sync()

	content, _ := os.ReadFile(tmpfile)
	output := string(content)

	// Debug should NOT appear, but info and warn should
	if strings.Contains(output, "debug msg") {
		t.Error("verbose mode should not log debug messages")
	}
	if !strings.Contains(output, "info msg") {
		t.Error("verbose mode should log info messages")
	}
	if !strings.Contains(output, "warn msg") {
		t.Error("verbose mode should log warn messages")
	}
}

func TestNew_DefaultMode(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "test.log")

	log, err := New(tmpfile, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer log.Sync()

	// Default mode should only log warn and error
	log.Debug("debug msg")
	log.Info("info msg")
	log.Warn("warn msg")
	log.Error("error msg")
	log.Sync()

	content, _ := os.ReadFile(tmpfile)
	output := string(content)

	if strings.Contains(output, "debug msg") {
		t.Error("default mode should not log debug messages")
	}
	if strings.Contains(output, "info msg") {
		t.Error("default mode should not log info messages")
	}
	if !strings.Contains(output, "warn msg") {
		t.Error("default mode should log warn messages")
	}
	if !strings.Contains(output, "error msg") {
		t.Error("default mode should log error messages")
	}
}
