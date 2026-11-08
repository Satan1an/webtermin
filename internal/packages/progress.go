package packages

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Progress is a snapshot of the package operation that is running now (or
// ran last), so the UI can show apt/dnf output while the POST is in flight.
type Progress struct {
	Running   bool      `json:"running"`
	Op        string    `json:"op"`
	Names     []string  `json:"names"`
	StartedAt time.Time `json:"started_at"`
	Lines     []string  `json:"lines"`
	Error     string    `json:"error,omitempty"`
}

const progressMaxLines = 400

type progressBuf struct {
	mu   sync.Mutex
	cur  Progress
	part []byte // unterminated tail of the last write
}

var progress progressBuf

// CurrentProgress returns a copy of the live progress buffer.
func CurrentProgress() Progress {
	progress.mu.Lock()
	defer progress.mu.Unlock()
	p := progress.cur
	p.Lines = append([]string(nil), progress.cur.Lines...)
	if len(progress.part) > 0 {
		p.Lines = append(p.Lines, string(progress.part))
	}
	return p
}

func (b *progressBuf) start(op string, names []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cur = Progress{Running: true, Op: op, Names: names, StartedAt: time.Now()}
	b.part = nil
}

func (b *progressBuf) end(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.part) > 0 {
		b.appendLine(string(b.part))
		b.part = nil
	}
	b.cur.Running = false
	if err != nil {
		b.cur.Error = err.Error()
	}
}

// Write feeds command output in line by line. apt uses \r for its download
// counters; treat it as a line break so the UI shows the latest state instead
// of one ever-growing line.
func (b *progressBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data := append(b.part, p...)
	for {
		i := bytes.IndexAny(data, "\r\n")
		if i < 0 {
			break
		}
		if line := strings.TrimRight(string(data[:i]), " "); line != "" {
			b.appendLine(line)
		}
		data = data[i+1:]
	}
	b.part = append([]byte(nil), data...)
	return len(p), nil
}

func (b *progressBuf) appendLine(line string) {
	// "(Reading database ... 5%)", "... 10%", … : keep only the latest step.
	if n := len(b.cur.Lines); n > 0 && percentPrefix(line) != "" && percentPrefix(line) == percentPrefix(b.cur.Lines[n-1]) {
		b.cur.Lines[n-1] = line
		return
	}
	b.cur.Lines = append(b.cur.Lines, line)
	if n := len(b.cur.Lines); n > progressMaxLines {
		b.cur.Lines = b.cur.Lines[n-progressMaxLines:]
	}
}

// percentPrefix returns the text before a trailing "NN%" counter, or "" if
// the line doesn't end in one.
func percentPrefix(line string) string {
	if !strings.HasSuffix(line, "%") {
		return ""
	}
	i := len(line) - 1
	for i > 0 && line[i-1] >= '0' && line[i-1] <= '9' {
		i--
	}
	if i == len(line)-1 {
		return ""
	}
	return line[:i]
}

// execLogged runs cmd with its output mirrored into the progress buffer. On
// failure the error carries the last lines, which is where apt puts the reason.
func execLogged(cmd *exec.Cmd, what string) error {
	var all bytes.Buffer
	cmd.Stdout = io.MultiWriter(&progress, &all)
	cmd.Stderr = cmd.Stdout
	if err := cmd.Run(); err != nil {
		lines := strings.Split(strings.TrimSpace(all.String()), "\n")
		if len(lines) > 15 {
			lines = lines[len(lines)-15:]
		}
		return fmt.Errorf("%s: %v: %s", what, err, strings.Join(lines, "\n"))
	}
	return nil
}
