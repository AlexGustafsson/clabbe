package ffmpeg

import (
	"io"
	"path"
)

var _ io.Reader = (*NormalizedAudioStream)(nil)

// NormalizedAudioStream uses FFMPEG to normalize an audio stream.
type NormalizedAudioStream struct {
	ffmpegErr error
	reader    io.Reader
}

func NewNormalizedAudioStream(reader io.Reader) (*NormalizedAudioStream, error) {
	ffmpeg, err := New(&Options{
		Stdin: reader,
		Arguments: func(endpoint string) []string {
			arguments := []string{
				"-re", "-i", "pipe:",
				"-filter:a", "loudnorm",
				"-c:a", "libopus",
				path.Join(endpoint, "audio.ogg"),
			}

			return arguments
		},
	})
	if err != nil {
		return nil, err
	}

	normalizedReader, normalizedWriter := io.Pipe()

	ffmpeg.OnStream = func(fileName, extension string, body io.ReadCloser) {
		defer body.Close()
		defer normalizedWriter.Close()
		io.Copy(normalizedWriter, body)
	}

	stream := &NormalizedAudioStream{
		reader: normalizedReader,
	}

	go func() {
		if err := ffmpeg.Run(); err != nil {
			stream.ffmpegErr = err
		}
	}()

	return stream, nil
}

func (s *NormalizedAudioStream) Read(p []byte) (int, error) {
	if s.ffmpegErr != nil {
		return 0, s.ffmpegErr
	}

	return s.reader.Read(p)
}
