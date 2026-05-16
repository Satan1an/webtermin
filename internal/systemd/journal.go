package systemd

import (
	"bufio"
	"context"
	"io"
	"os/exec"
)

// TailJournal streams journalctl output for a given unit. Caller reads from the
// returned reader; cancel ctx to stop. Uses an argv slice — no shell.
func TailJournal(ctx context.Context, unit string, lines int) (io.ReadCloser, error) {
	if !ValidUnitName(unit) {
		return nil, errInvalidUnit
	}
	if lines <= 0 || lines > 1000 {
		lines = 200
	}
	cmd := exec.CommandContext(ctx, "journalctl",
		"-u", unit,
		"--no-pager",
		"-n", itoa(lines),
		"-f",
		"-o", "short-iso",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &cmdReader{cmd: cmd, r: stdout, br: bufio.NewReader(stdout)}, nil
}

var errInvalidUnit = invalidUnit{}

type invalidUnit struct{}

func (invalidUnit) Error() string { return "invalid unit name" }

type cmdReader struct {
	cmd *exec.Cmd
	r   io.ReadCloser
	br  *bufio.Reader
}

func (c *cmdReader) Read(p []byte) (int, error) { return c.br.Read(p) }

func (c *cmdReader) Close() error {
	_ = c.r.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
