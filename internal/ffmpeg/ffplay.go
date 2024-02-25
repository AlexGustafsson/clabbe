package ffmpeg

import (
	"bufio"
	"io"
	"log/slog"
	"os/exec"
	"syscall"
)

var _ io.WriteCloser = (*Player)(nil)

type Player struct {
	stdinWriter io.WriteCloser
	cmd         *exec.Cmd
}

// Play plays a stream provided by the reader using ffplay.
// Useful to test streams interactively.
func NewPlayer() (*Player, error) {
	cmd := exec.Command("ffplay", "-autoexit", "-")
	// Run ffplay in a separate process group to keep it from listening on
	// signals sent to the host process.
	// NOTE: Does not work on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{Pgid: 0, Setpgid: true}

	// Read content from stdin
	stdinReader, stdinWriter := io.Pipe()
	cmd.Stdin = stdinReader

	// Output ffplay's logs as debug logs
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			slog.Debug(scanner.Text())
		}
	}()

	slog.Debug("Starting ffplay")
	go func() {
		cmd.Run()
	}()

	return &Player{
		stdinWriter: stdinWriter,
		cmd:         cmd,
	}, nil
}

// Write implements io.Writer.
func (p *Player) Write(d []byte) (n int, err error) {
	return p.stdinWriter.Write(d)
}

// Close implements io.WriteCloser.
func (p *Player) Close() error {
	return p.stdinWriter.Close()
}
