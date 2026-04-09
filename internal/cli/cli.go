package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type exitError struct {
	code   int
	err    error
	silent bool
}

func (e *exitError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func Run(ctx context.Context, args []string, logger *slog.Logger) int {
	return Execute(ctx, args, logger, os.Stdout, os.Stderr)
}

func Execute(ctx context.Context, args []string, logger *slog.Logger, stdout, stderr io.Writer) int {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(stderr, nil))
	}

	root := newRootCommand(ctx, logger, stdout, stderr)
	root.SetArgs(args)

	if err := root.ExecuteContext(ctx); err != nil {
		var cliErr *exitError
		if errors.As(err, &cliErr) {
			if !cliErr.silent && cliErr.err != nil {
				_, _ = fmt.Fprintln(stderr, cliErr.err)
			}
			return cliErr.code
		}

		_, _ = fmt.Fprintln(stderr, err)
		if strings.Contains(err.Error(), "unknown command") {
			_, _ = fmt.Fprintln(stderr)
			_ = root.Help()
			return 2
		}
		return 1
	}

	return 0
}

func silentExit(code int) error {
	return &exitError{code: code, silent: true}
}

func usageErrorf(format string, args ...any) error {
	return &exitError{
		code: 2,
		err:  fmt.Errorf(format, args...),
	}
}

func commandErrorf(format string, args ...any) error {
	return &exitError{
		code: 1,
		err:  fmt.Errorf(format, args...),
	}
}
