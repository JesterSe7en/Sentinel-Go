// Copyright (c) 2025 Alexander Chan
// SPDX-License-Identifier: MIT

// Package logger provides logging functionality for the photothingy tool. It wraps zap logger
// to provide a simple interface for structured logging.
package logger

import (
	"fmt"

	"go.uber.org/zap"
)

type Logger struct {
	logger *zap.SugaredLogger
}

func New(filename string, debug bool, verbose bool) (*Logger, error) {
	cfg := zap.NewDevelopmentConfig()
	if !debug {
		cfg.DisableStacktrace = true
	}
	if filename == "" {
		cfg.OutputPaths = []string{"stderr"}
		cfg.ErrorOutputPaths = []string{"stderr"}
	} else {
		cfg.OutputPaths = []string{filename}
		cfg.ErrorOutputPaths = []string{filename}
	}

	if debug {
		// debug + info + warn + error
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else if verbose {
		// info + warn + error
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	} else {
		// error + warn
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	}

	var err error
	l, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	return &Logger{
		logger: l.Sugar(),
	}, nil
}

func (l *Logger) Info(msg string, args ...any) {
	l.logger.Infow(msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warnw(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.logger.Errorw(msg, args...)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debugw(msg, args...)
}

func (l *Logger) Fatal(msg string, args ...any) {
	l.logger.Fatalw(msg, args...)
}

func (l *Logger) Sync() {
	_ = l.logger.Sync()
}
