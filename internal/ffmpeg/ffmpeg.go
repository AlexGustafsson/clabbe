package ffmpeg

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"path"
	"strings"
	"syscall"

	"golang.org/x/sync/errgroup"
)

type Options struct {
	Arguments func(endpoint string) []string
	Stdin     io.Reader
}

type FFMPEG struct {
	OnStream func(fileName string, extension string, body io.ReadCloser)

	stdin io.WriteCloser

	cmd *exec.Cmd

	listener net.Listener

	wg errgroup.Group
}

func New(options *Options) (*FFMPEG, error) {
	// Listen on a random port for data from FFMPEG
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("ffmpeg", options.Arguments("http://"+listener.Addr().String())...)
	// Run ffmpeg in a separate process group to keep it from listening on
	// signals sent to the host process.
	// NOTE: Does not work on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{Pgid: 0, Setpgid: true}

	ffmpeg := &FFMPEG{
		cmd: cmd,

		listener: listener,
	}

	cmd.Stdin = options.Stdin

	return ffmpeg, nil
}

// Run will start FFMPEG and wait for it to complete.
func (f *FFMPEG) Run() error {
	server := &http.Server{
		Handler: http.HandlerFunc(f.serveHTTP),
	}

	reader, writer := io.Pipe()
	f.cmd.Stdout = writer
	f.cmd.Stderr = writer

	// Output FFMPEG's logs as debug logs
	f.wg.Go(func() error {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			slog.Debug(scanner.Text(), slog.String("component", "ffmpeg"))
		}
		return nil
	})

	// Run the command. When it exits, close the server
	f.wg.Go(func() error {
		defer server.Close()
		defer writer.Close()

		err := f.cmd.Run()
		if err != nil && err.Error() == "exit status 255" {
			return nil
		}
		return err
	})

	// Start the server and wait for it to exit (when FFMPEG exits)
	err := server.Serve(f.listener)
	if err != http.ErrServerClosed {
		f.cmd.Process.Kill()
		return err
	}
	return nil
}

// Stop will tell FFMPEG to stop gracefully.
// Waits for FFMPEG to exit.
func (s *FFMPEG) Stop() error {
	s.cmd.Process.Signal(syscall.SIGINT)
	s.stdin.Close()
	return s.wg.Wait()
}

// Wait waits for FFMPEG to exit.
func (s *FFMPEG) Wait() error {
	return s.wg.Wait()
}

// Kill will immediately kill FFMPEG.
func (s *FFMPEG) Kill() {
	s.cmd.Process.Kill()
}

func (s *FFMPEG) Stdin() io.WriteCloser {
	return s.stdin
}

func (f *FFMPEG) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	fileName := strings.TrimPrefix(r.URL.Path, "/")
	extension := path.Ext(fileName)

	f.OnStream(fileName, extension, r.Body)
}
