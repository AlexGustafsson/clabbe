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
				// Make sure the audio is resampled to 2 channel, 48kHz, 96kbps
				"-ac", "2", "-ar", "48000", "-b:a", "96000",
				// Normalize according to Spotify's guidelines
				// TODO: For now, don't normalize audio. IT can create a lot of noise
				// at the start of songs. The dynamic normalization filter makes the
				// sound sound wobbly
				// "-filter:a", "loudnorm=I=-14:TP=-2.0:LRA=7.0:linear=true:print_format=summary",
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
			normalizedWriter.Close()
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
