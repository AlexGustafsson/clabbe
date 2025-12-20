package ytdlp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
)

type Error struct {
	ExitCode int
	Stderr   string
}

func (e Error) Error() string {
	return fmt.Sprintf("yt-dlp: exit code %d", e.ExitCode)
}

// Stream uses yt-dlp to stream opus audio in a webm container to w.
func Stream(ctx context.Context, url string, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "yt-dlp", "--quiet", "--no-playlist", "-f", "ba[ext=webm][acodec=opus]", "-o", "-", url)

	cmd.Stdout = w

	var buffer bytes.Buffer
	cmd.Stderr = &buffer

	if err := cmd.Run(); err != nil {
		return Error{
			ExitCode: cmd.ProcessState.ExitCode(),
			Stderr:   buffer.String(),
		}
	}

	return nil
}
