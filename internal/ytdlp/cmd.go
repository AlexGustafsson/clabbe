package ytdlp

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

// Stream uses yt-dlp to stream opus audio in a webm container to w.
func Stream(ctx context.Context, url string, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "yt-dlp", "--no-playlist", "-f", "ba[ext=webm][acodec=opus]", "-o", "-", url)

	cmd.Stdout = w

	var buffer bytes.Buffer
	cmd.Stderr = &buffer

	err := cmd.Run()
	return err
}
