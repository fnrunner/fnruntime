package fnruntime

import (
	"bytes"
	"context"
	goerrors "errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/fnrunner/fnruntime/internal/printer"
	fnresultv1alpha1 "github.com/fnrunner/fnsyntax/apis/fnresult/v1alpha1"
)

type ExecFn struct {
	// Path is the os specific path to the executable
	// file. It can be relative or absolute.
	Path string
	// Args are the arguments to the executable
	Args []string

	Env map[string]string
	// Container function will be killed after this timeour.
	// The default value is 5 minutes.
	Timeout time.Duration
	// FnResult is used to store the information about the result from
	// the function.
	FnResult *fnresultv1alpha1.Result
}

func (f *ExecFn) SvcRun(ctx context.Context) error { return nil }

// Run runs the executable file which reads the input from r and
// writes the output to w.
func (f *ExecFn) FnRun(ctx context.Context, r io.Reader, w io.Writer) error {
	// setup exec run timeout
	timeout := defaultLongTimeout
	if f.Timeout != 0 {
		timeout = f.Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.Path, f.Args...)

	errSink := bytes.Buffer{}
	cmd.Stdin = r
	cmd.Stdout = w
	cmd.Stderr = &errSink

	for k, v := range f.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%v=%v", k, v))
	}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if goerrors.As(err, &exitErr) {
			return &ExecError{
				OriginalErr:    exitErr,
				ExitCode:       exitErr.ExitCode(),
				Stderr:         errSink.String(),
				TruncateOutput: printer.TruncateOutput,
			}
		}
		return fmt.Errorf("unexpected function error: %w", err)
	}

	if errSink.Len() > 0 {
		f.FnResult.Stderr = errSink.String()
	}

	return nil
}
