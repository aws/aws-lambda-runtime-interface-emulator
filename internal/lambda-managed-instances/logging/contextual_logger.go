// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/interop"
	"github.com/aws/aws-lambda-runtime-interface-emulator/internal/lambda-managed-instances/rapid/model"
)

func init() {
	w := io.Writer(os.Stderr)

	slog.SetDefault(CreateNewLogger(slog.LevelInfo, w))
}

type loggerKey int

const (
	ctxLoggerKey loggerKey = iota
)

func CreateNewLogger(level slog.Level, w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   slog.TimeKey,
					Value: slog.StringValue(a.Value.Time().UTC().Format("2006-01-02T15:04:05.0000Z")),
				}
			}
			return a
		},
	}))
}

func WithFields(ctx context.Context, args ...any) context.Context {
	logger := FromContext(ctx)
	logger = logger.With(args...)
	return context.WithValue(ctx, ctxLoggerKey, logger)
}

func WithInvokeID(ctx context.Context, invokeID interop.InvokeID) context.Context {
	return WithFields(ctx, interop.RequestIdProperty, invokeID)
}

func Debug(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Debug(msg, args...)
}

func Info(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Info(msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Warn(msg, args...)
}

func Error(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).Error(msg, args...)
}

func Err(ctx context.Context, msg string, err model.AppError) {
	level := slog.LevelWarn
	if err.Source() != model.ErrorSourceRuntime {
		level = slog.LevelError
	}
	FromContext(ctx).Log(ctx, level, msg, "err", err)
}

func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(ctxLoggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
