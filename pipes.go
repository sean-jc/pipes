package pipes

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"syscall"
)

// Exec executes a single command, optionally reading data from stdin,
// writing the output to stdout and writing Stderr output to stderr.
// Stdout/Stderr are discarded if stdout/stderr are nil.  Returns an error
// containing the command that failed as well as the system error string.
func Exec(cmd *exec.Cmd, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if stdin != nil {
		cmd.Stdin = stdin
	}
	if stdout == nil {
		stdout = ioutil.Discard
	}
	if stderr == nil {
		stderr = ioutil.Discard
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("%s %s", cmd.Path, err.Error())
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("%s %s", cmd.Path, err.Error())
	}
	return nil
}

// ExecE runs a single command together, optionally reading data from
// stdin and writing the output to stdout.  Stdout is discarded if
// stdout is nil.  Returns an error containing the command that failed,
// the system error string and any information captured from Stderr.
func ExecE(cmd *exec.Cmd, stdin io.Reader, stdout io.Writer) error {
	var stderr bytes.Buffer

	if err := Exec(cmd, stdin, stdout, &stderr); err != nil {
		return fmt.Errorf("%s - %s", err.Error(), stderr.String())
	}
	return nil
}

// ExecO executes a single command, optionally reading data from stdin.
// Returns the command's Stdout as a byte slice, and an error containing
// the command that failed, the system error string and any information
// captured from Stderr.
func ExecO(cmd *exec.Cmd, stdin io.Reader) ([]byte, error) {
	var stdout bytes.Buffer

	if err := ExecE(cmd, stdin, &stdout); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// ExecStdin executes a single command, optionally reading data from stdin.
// Output from Stdout is dicarded.  Returns an error containing the system
// error string and any information captured from Stderr.
func ExecStdin(cmd *exec.Cmd, stdin io.Reader) error {
	return ExecE(cmd, stdin, nil)
}

// ExecStdout executes a single command, writing its output to stdout.
// Returns an error containing the command that failed, the system error
// string and any information captured from Stderr.
func ExecStdout(cmd *exec.Cmd, stdout io.Writer) error {
	return ExecE(cmd, nil, stdout)
}

// ExecPipeline pipes several commands together, optionally reading data from
// stdin for the first command, writing the output from the last command to
// stdout and writing all commands' Stderr output to stderr.  Stdout and Stderr
// are discarded if stdout or stderr are nil, respectively.  Returns an error
// containing the command that failed as well as the system error string.
func ExecPipeline(cmds []*exec.Cmd, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	var err error

	// Require at least one command
	if len(cmds) < 1 {
		return fmt.Errorf("No commands provided to ExecPipeline")
	}

	// Connect the optional input to the first command's stdin if necessary
	if stdin != nil {
		cmds[0].Stdin = stdin
	}
	if stdout == nil {
		stdout = ioutil.Discard
	}
	if stderr == nil {
		stderr = ioutil.Discard
	}

	last := len(cmds) - 1
	for i, cmd := range cmds[:last] {
		// Connect each command's stdin to the previous command's stdout
		if cmds[i+1].Stdin, err = cmd.StdoutPipe(); err != nil {
			return fmt.Errorf("%s %s", cmd.Path, err)
		}
		// Connect each command's Stderr to the stderr writer
		cmd.Stderr = stderr
	}

	// Connect the output and error for the last command
	cmds[last].Stdout, cmds[last].Stderr = stdout, stderr

	// Start each command; defer a function to conditionally kill
	// each started process if any process in the pipeline fails.
	for _, cmd := range cmds {
		if err = cmd.Start(); err != nil {
			return fmt.Errorf("%s %s", cmd.Path, err.Error())
		}

		kill := cmd
		defer func() {
			if err != nil && kill.Process != nil {
				kill.Process.Signal(syscall.SIGKILL)
				kill.Process.Wait()
			}
		}()
	}

	// Wait for each command to complete
	for _, cmd := range cmds {
		if err = cmd.Wait(); err != nil {
			return fmt.Errorf("%s %s", cmd.Path, err.Error())
		}
	}

	// Success!
	return nil
}

// ExecPipelineE pipes several commands together, optionally reading data from
// stdin for the first command and writing the output from the last command
// to stdout.  Output from Stdout is discarded if stdout is nil.  Returns an
// error containing the command that failed, the system error string, and any
// information captured from Stderr.
func ExecPipelineE(cmds []*exec.Cmd, stdin io.Reader, stdout io.Writer) error {
	var stderr bytes.Buffer

	if err := ExecPipeline(cmds, stdin, stdout, &stderr); err != nil {
		return fmt.Errorf("%s - %s", err.Error(), stderr.String())
	}
	return nil
}

// ExecPipelineO pipes several commands together, reading data from optional
// stdin for the first command.  Returns the Stdout from the last command as
// a byte slice, and if any command fails, an error containing the command
// that failed, the system error string, and any information captured from
// Stderr.
func ExecPipelineO(cmds []*exec.Cmd, stdin io.Reader) ([]byte, error) {
	var stdout bytes.Buffer
	err := ExecPipelineE(cmds, stdin, &stdout)
	return stdout.Bytes(), err
}
