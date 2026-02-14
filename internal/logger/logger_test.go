package logger

import (
	"os"
	"testing"
)

func TestNew_DefaultOutput(t *testing.T) {
	log, err := New("", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log == nil {
		t.Fatal("logger should not be nil")
	}
	log.Sync()
}

func TestNew_FileOutput(t *testing.T) {
	tmpFile := "/tmp/sentinel-test.log"
	log, err := New(tmpFile, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log == nil {
		t.Fatal("logger should not be nil")
	}
	log.Sync()
	os.Remove(tmpFile)
}

func TestNew_InvalidPath(t *testing.T) {
	_, err := New("/invalid/path/that/does/not/exist/test.log", false, false)
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestNew_DebugMode(t *testing.T) {
	log, err := New("", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log == nil {
		t.Fatal("logger should not be nil")
	}
	log.Sync()
}

func TestNew_VerboseMode(t *testing.T) {
	log, err := New("", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log == nil {
		t.Fatal("logger should not be nil")
	}
	log.Sync()
}

func TestLogger_Info(t *testing.T) {
	log, err := New("", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log.Info("test message", "key", "value")
	log.Sync()
}

func TestLogger_Warn(t *testing.T) {
	log, err := New("", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log.Warn("test warning", "key", "value")
	log.Sync()
}

func TestLogger_Error(t *testing.T) {
	log, err := New("", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log.Error("test error", "key", "value")
	log.Sync()
}

func TestLogger_Debug(t *testing.T) {
	log, err := New("", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log.Debug("test debug", "key", "value")
	log.Sync()
}

func TestLogger_Fatal(t *testing.T) {
	// Fatal calls os.Exit which cannot be recovered in tests
	// This test just verifies the method exists and can be called
	log, err := New("", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = log.Fatal // Verify method exists
}

func TestLogger_Sync(t *testing.T) {
	log, err := New("", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log.Sync()
}
