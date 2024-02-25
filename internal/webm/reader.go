package webm

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/AlexGustafsson/clabbe/internal/ebml"
)

type Frame struct {
	Track    uint64
	Timecode uint16
	Flags    byte
	Payload  []byte
}

// Reader reads frames from a webm stream.
type Reader struct {
	reader *ebml.Reader
}

// NewReader creates a new Reader that will read from reader.
func NewReader(reader io.Reader) *Reader {
	return &Reader{
		reader: ebml.NewReader(reader),
	}
}

// Read reads the next frame.
func (r *Reader) Read() (*Frame, error) {
	for {
		tag, size, err := r.reader.NextElement()
		if err != nil {
			return nil, err
		}

		// SEE: https://darkcoding.net/software/reading-mediarecorders-webm-opus-output/
		// SEE: https://www.matroska.org/technical/elements.html
		// SEE: https://github.com/ietf-wg-cellar/ebml-specification/blob/master/specification.markdown#ebml-header-elements
		// SEE: https://www.ietf.org/archive/id/draft-lhomme-cellar-matroska-04.txt
		switch tag {
		// /EBML
		case 0x1A45dfa3:
			// Master element, continue
		// /EBML/DocType
		case 0x4282:
			c := make([]byte, size)
			if _, err := r.reader.Read(c); err != nil {
				return nil, err
			}

			if string(c) != "webm" {
				return nil, fmt.Errorf("webm: ebml container does not contain webm")
			}
		// /Segment
		case 0x18538067:
			// Master element, continue
		// /Segment/Cluster
		case 0x1f43b675:
			// Master element, continue
		// /Segment/Cluster/BlockGroup
		case 0xa0:
			// Master element, continue
		// /Segment/Cluster/SimpleBlock
		case 0xa3:
			fallthrough
		// /Segment/Cluster/BlockGroup/Block
		case 0xa1:
			r := r.reader.Reader()
			track, _, err := ebml.ReadVINT(r)
			if err != nil {
				return nil, err
			}

			var timecode uint16
			if err := binary.Read(r, binary.BigEndian, &timecode); err != nil {
				return nil, err
			}

			var flags [1]byte
			if _, err := r.Read(flags[:]); err != nil {
				return nil, err
			}

			payload, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}

			return &Frame{
				Track:    track,
				Timecode: timecode,
				Flags:    flags[0],
				Payload:  payload,
			}, nil
		default:
			// Discard unimportant segments to progress the reader
			r.reader.Discard()
		}
	}
}
