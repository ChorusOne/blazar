package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"blazar/internal/pkg/errors"
	"blazar/internal/pkg/log"
)

// cmdErr is a custom error that holds information about potential cancellation or context timeout
// See: https://github.com/golang/go/issues/21880
type cmdErr struct {
	err    error
	ctxErr error
}

func (e cmdErr) Is(target error) bool {
	switch target {
	case context.DeadlineExceeded, context.Canceled:
		return e.ctxErr == context.DeadlineExceeded || e.ctxErr == context.Canceled
	}
	return false
}

func (e cmdErr) Error() string {
	return e.err.Error()
}

// CheckOutputWithDeadline returns stdout, stderr, error
func CheckOutputWithDeadline(ctx context.Context, deadline time.Duration, envVars []string, command string, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Append custom env vars to current-process env vars
	// in case of conflict, last entry wins.
	cmd.Env = append(os.Environ(), envVars...)

	// After cmd.Start(), when a deadline is triggered, cmd.Wait
	// waits for the provided stdout and stderr buffers to close.
	// If a grandchild-subprocess is spawned that inherits this stdout and stderr
	// and it gets stuck, cmd.Wait will wait forever. cmd.WaitDelay
	// allowed us to configure another deadline after which cmd.Wait
	// will force close the stdout and stderr buffers. However, this
	// still doesn't kill the stuck subprocess. Hence we do the following:

	// Set the process group ID so that we can kill the process and its children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pid := cmd.Process.Pid
		if pid <= 0 {
			return fmt.Errorf("process group: kill argument %d is invalid", pid)
		}

		// to kill all child processes
		err := syscall.Kill(pid, syscall.SIGKILL)
		if err != nil {
			return errors.Wrapf(err, "process group: kill syscall failed")
		}
		return nil
	}

	// Lets configure WaitDelay anyway
	cmd.WaitDelay = time.Second

	if err := cmd.Run(); err != nil {
		return &stdout, &stderr, cmdErr{
			err: errors.Wrapf(
				err,
				"command failed, cmd: %q, args: %s, deadline: %s, timed out: %t",
				command, args, deadline.String(), ctx.Err() == context.DeadlineExceeded,
			),
			ctxErr: ctx.Err(),
		}
	}
	return &stdout, &stderr, nil
}

func ExecuteWithDeadlineAndLog(ctx context.Context, deadline time.Duration, envVars []string, command string, args ...string) error {
	logger := log.FromContext(ctx)
	logger.Infof("Executing command %q %s", command, args)

	stdout, stderr, err := CheckOutputWithDeadline(ctx, deadline, envVars, command, args...)
	logger.Infof("Command %q args: %s stdout:\n%s", command, args, stdout.String())
	logger.Infof("Command %q args: %s stderr:\n%s", command, args, stderr.String())
	return err
}
