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
				"-i", "pipe:",
				// Make sure the audio is resampled to 2 channel, 48kHz
				"-ac", "2", "-ar", "48000",
				"-filter:a", "loudnorm",
				// OPUS supports up to 60ms windows. FFMPEG likes 1s by default.
				// Use 20ms as too high windows will not work with Discord
				"-page_duration", "20000",
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
