package webm

import (
	"bytes"
	_ "embed" // Embed files
	"io"
	"testing"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
	"github.com/stretchr/testify/require"
)

// TestFile is a 1s long OPUS-encoded sine wave in a WebM container.
// Generated using:
// ffmpeg -f lavfi -i "sine=r=44000:frequency=440:duration=1" -ac 2 -c:a libopus test.webm
//
//go:embed test.webm
var TestFile []byte

func TestReader(t *testing.T) {
	reader := NewReader(bytes.NewReader(TestFile))

	// As plain OPUS has very little support in players, mux it back to an ogg
	// file
	w, err := oggwriter.New("out.ogg", 44000, 2)
	require.NoError(t, err)
	defer w.Close()

	for {
		frame, err := reader.Read()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		w.WriteRTP(&rtp.Packet{
			Header: rtp.Header{
				Timestamp: uint32(frame.Timecode),
			},
			Payload: frame.Payload,
		})
	}
}
