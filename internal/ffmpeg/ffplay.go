package ffmpeg

import (
	"bufio"
	"io"
	"log/slog"
	"os/exec"
	"syscall"
)

// Play plays a stream provided by the reader using ffplay.
// Useful to test streams interactively.
func Play(reader io.Reader) error {
	cmd := exec.Command("ffplay", "-autoexit", "-")
	// Run ffplay in a separate process group to keep it from listening on
	// signals sent to the host process.
	// NOTE: Does not work on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{Pgid: 0, Setpgid: true}

	// Read content from stdin
	cmd.Stdin = reader

	// Output ffplay's logs as debug logs
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer
	go func() error {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			slog.Debug(scanner.Text())
		}
		return nil
	}()
	defer writer.Close()

	slog.Debug("Starting ffplay")
	return cmd.Run()
}
