package dummy

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
)

func execCommand(program string, args ...string) (stdout, stderr []byte, err error) {
	var (
		cmd       = exec.Command(program, args...)
		stdoutBuf bytes.Buffer
		stderrBuf bytes.Buffer
	)

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	log.Printf("exec %s %v", program, args)

	if err := cmd.Run(); err != nil {
		if cmd.Process == nil {
			return nil, nil, fmt.Errorf("process failed to start for %s %v: %v", program, args, err)
		}

		log.Printf("process %d for %s %v: stderr: %s", cmd.Process.Pid, program, args, stderrBuf.Bytes())

		return stdoutBuf.Bytes(), stderrBuf.Bytes(), fmt.Errorf("process %d finished with failure for %s %v: %v",
			cmd.Process.Pid, program, args, err)
	}

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), nil
}

func execCommandExitCode(program string, args ...string) (int, error) {
	cmd := exec.Command(program, args...)
	log.Printf("exec %s %v", program, args)

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}

		return -1, fmt.Errorf("failed to exec %s %v: %v", program, args, err)
	}

	return 0, nil
}

func execCommandErr(program string, args ...string) error {
	_, _, err := execCommand(program, args...)
	return err
}
