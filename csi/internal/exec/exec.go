package exec

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync/atomic"

	"github.com/gman0/dummy-fuse-csi/csi/internal/log"
)

// This file only provides wrappers around "os/exec" and logs the executed commands.

var (
	// Counter value used for pairing pre- and post-exec log messages.
	execCounter uint64
)

func FmtLogMsg(execID uint64, msg string) string {
	return fmt.Sprintf("Exec-ID %d: %s", execID, msg)
}

func Run(cmd *exec.Cmd) error {
	c := atomic.AddUint64(&execCounter, 1)
	log.InfofDepth(2, FmtLogMsg(c, "Running command env=%v prog=%s cmd=%v"), cmd.Env, cmd.Path, cmd.Args)

	err := cmd.Run()
	log.InfofDepth(2, FmtLogMsg(c, "Process exited: %s"), cmd.ProcessState)

	if err != nil {
		log.ErrorfDepth(2, FmtLogMsg(c, "Error: %v"), err)
	}

	return err
}

func Output(cmd *exec.Cmd) ([]byte, error) {
	c := atomic.AddUint64(&execCounter, 1)
	log.InfofDepth(2, FmtLogMsg(c, "Running command env=%v prog=%s args=%v"), cmd.Env, cmd.Path, cmd.Args)

	out, err := cmd.Output()
	log.InfofDepth(2, FmtLogMsg(c, "Process exited: %s"), cmd.ProcessState)

	if err != nil {
		log.ErrorfDepth(2, FmtLogMsg(c, "Error: %v"), err)
	}

	return out, err
}

func CombinedOutput(cmd *exec.Cmd) ([]byte, error) {
	c := atomic.AddUint64(&execCounter, 1)
	log.InfofDepth(2, FmtLogMsg(c, "Running command env=%v prog=%s args=%v"), cmd.Env, cmd.Path, cmd.Args)

	out, err := cmd.CombinedOutput()
	log.InfofDepth(2, FmtLogMsg(c, "Process exited: %s"), cmd.ProcessState)

	if err != nil {
		log.ErrorfDepth(2, FmtLogMsg(c, "Error: %v; Output: %s"), err, out)
	}

	return out, err
}

func RunAndLogCombined(cmd *exec.Cmd) error {
	return RunAndDoCombined(cmd, func(execID uint64, line string) {
		log.Infof(FmtLogMsg(execID, line))
	})
}

func RunAndDoCombined(cmd *exec.Cmd, eachCombinedOutLine func(execID uint64, line string)) error {
	c := atomic.AddUint64(&execCounter, 1)
	log.InfofDepth(2, FmtLogMsg(c, "Running command env=%v prog=%s args=%v"), cmd.Env, cmd.Path, cmd.Args)

	rd, wr := io.Pipe()
	defer rd.Close()
	defer wr.Close()

	cmd.Stdout = wr
	cmd.Stderr = wr

	go func() {
		scanner := bufio.NewScanner(rd)
		for scanner.Scan() {
			eachCombinedOutLine(c, scanner.Text())
		}
	}()

	return cmd.Run()
}
