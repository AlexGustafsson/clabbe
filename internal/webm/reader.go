package webm

import (
	"bufio"
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

type Reader struct {
	reader *ebml.Reader
}

func NewReader(reader *bufio.Reader) *Reader {
	return &Reader{
		reader: ebml.NewReader(reader),
	}
}

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
		// /EBML/EBMLVersion
		case 0x4286:
			// Version element, discard
			r.reader.Discard()
		// /EBML/EBMLReadVersion
		case 0x42F7:
			r.reader.Discard()
		// /EBML/EBMLMaxIDLength
		case 0x42F2:
			r.reader.Discard()
		// /EBML/EBMLMaxSizeLength
		case 0x42F3:
			r.reader.Discard()
		// /EBML/DocType
		case 0x4282:
			c := make([]byte, size)
			if _, err := r.reader.Read(c); err != nil {
				return nil, err
			}

			if string(c) != "webm" {
				return nil, fmt.Errorf("webm: ebml container is not webm")
			}
		// /EBML/DocTypeVersion
		case 0x4287:
			r.reader.Discard()
		// /EBML/DocTypeReadVersion
		case 0x4285:
			r.reader.Discard()
		// /EBML/DocTypeExtension
		case 0x4281:
			r.reader.Discard()
		// /EBML/DocTypeExtensionName
		case 0x4284:
			r.reader.Discard()
		// /Segment
		case 0x18538067:
			// Master element, continue
		// /Segment/SeekHead
		case 0x114d9b74:
			r.reader.Discard()
		// /Segment/Info
		case 0x1549a966:
			r.reader.Discard()
		// /Segment/Tracks
		case 0x1654ae6b:
			// Master element, continue
		// /Segment/Tags
		case 0x1254c367:
			// Master element, continue
		// /Segment/Tags/Tag
		case 0x7373:
			// Master element, continue
		// /Segment/Tags/Tag/Targets
		case 0x63c0:
			// Master element, continue
		// /Segment/Tags/Tag/Targets/SimpleTag
		case 0x67c8:
			// Master element, continue
		// /Segment/Tags/Tag/Targets/SimpleTag/TagName
		case 0x45a3:
			r.reader.Discard()
		// /Segment/Tags/Tag/Targets/SimpleTag/TagString
		case 0x4487:
			r.reader.Discard()
		// /Segment/Tags/Tag/Targets/SimpleTag/TagTrackUID
		case 0x63c5:
			r.reader.Discard()
		// /Segment/Cluster
		case 0x1f43b675:
			// Master element, continue
		// /Segment/Cues
		case 0x1c53bb6b:
			// Master element, continue
		// /Segment/Cluster/SimpleBlock
		case 0xa3:
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
		// /Segment/Cluster/Timecode
		case 0xe7:
			r.reader.Discard()
		// /Segment/Cues/CuePoint
		case 0xbb:
			r.reader.Discard()
		// /Segment/Cluster/BlockGroup
		case 0xa0:
			// Master element, continue
		// /Segment/Tracks/TrackEntry
		case 0xae:
			// Master element, continue
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
		// /Segment/Tracks/TrackEntry/TrackNumber
		case 0xd7:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/TrackUID
		case 0x73c5:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/FlagLacing
		case 0x9c:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/Language
		case 0x22b59c:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/FlagDefault
		case 0x88:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/CodecID
		case 0x86:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/CodecDelay
		case 0x56aa:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/SeekPreRoll
		case 0x56bb:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/TrackType
		case 0x83:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/Block
		case 0xe1:
			// Master element, continue
		// /Segment/Tracks/TrackEntry/CodecPrivate
		case 0x63a2:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/Audio/Channels
		case 0x9f:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/Audio/SamplingFrequency
		case 0xb5:
			r.reader.Discard()
		// /Segment/Tracks/TrackEntry/Audio/BitDepth
		case 0x6264:
			r.reader.Discard()
		// /Segment/Cluster/BlockGroup/DiscardPadding
		case 0x75a2:
			r.reader.Discard()
		// Void
		case 0xec:
			r.reader.Discard()
		default:
			r.reader.Discard()
			fmt.Printf("webm: unexpected element: 0x%x\n", tag)
		}
	}
}
