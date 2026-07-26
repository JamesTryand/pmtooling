// Package git wraps the git CLI via os/exec. go-git is not used because it
// has no support for linked worktrees, which this tool depends on entirely.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ExitError is returned by Run when git executed but exited non-zero.
type ExitError struct {
	Args   []string
	Code   int
	Stderr string
}

func (e *ExitError) Error() string {
	msg := fmt.Sprintf("git %s: exit status %d", strings.Join(e.Args, " "), e.Code)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

// RunRaw executes git with args in dir and returns trimmed stdout and the
// process exit code. err is non-nil only when git itself could not be
// executed (e.g. not found) — a non-zero exit code is reported via code,
// not err, so callers that need to distinguish "not found" (exit 1) from
// other outcomes (e.g. show-ref --verify --quiet) can do so.
func RunRaw(dir string, args ...string) (stdout string, code int, err error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	if runErr == nil {
		return strings.TrimSpace(stdoutBuf.String()), 0, nil
	}
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		return strings.TrimSpace(stdoutBuf.String()), exitErr.ExitCode(), nil
	}
	return "", -1, fmt.Errorf("git %s: %w", strings.Join(args, " "), runErr)
}

// Run executes git with args in dir and returns trimmed stdout. A non-zero
// exit code is reported as an *ExitError including captured stderr.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	stdout := strings.TrimSpace(stdoutBuf.String())
	if runErr == nil {
		return stdout, nil
	}
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		return "", &ExitError{Args: args, Code: exitErr.ExitCode(), Stderr: strings.TrimSpace(stderrBuf.String())}
	}
	return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), runErr)
}

// ReadBlob returns the raw, untrimmed content of a blob (e.g.
// "<ref>:README.md" or a bare blob SHA) via `git show`. Unlike Run/RunRaw
// (which trim output — fine for status codes and single-line values but
// wrong for file content), this preserves exact bytes, needed by callers
// that re-render and recommit the content.
func ReadBlob(dir, treeish string) ([]byte, error) {
	cmd := exec.Command("git", "show", treeish)
	cmd.Dir = dir
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, &ExitError{Args: []string{"show", treeish}, Code: exitErr.ExitCode(), Stderr: strings.TrimSpace(stderrBuf.String())}
		}
		return nil, fmt.Errorf("git show %s: %w", treeish, err)
	}
	return stdoutBuf.Bytes(), nil
}

// Lines splits git output that uses one result per line, returning nil
// (not a single empty-string element) for empty output.
func Lines(output string) []string {
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

// RunWithStdin executes git with args in dir, feeding stdin to the
// process, and returns trimmed stdout. Used by the plumbing commands
// (hash-object, mktree) that read their payload from stdin.
func RunWithStdin(dir string, stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	stdout := strings.TrimSpace(stdoutBuf.String())
	if runErr == nil {
		return stdout, nil
	}
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		return "", &ExitError{Args: args, Code: exitErr.ExitCode(), Stderr: strings.TrimSpace(stderrBuf.String())}
	}
	return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), runErr)
}
