package ebml

import (
	"fmt"
	"io"
	"math/bits"
)

type ElementHeader struct {
	// Tag is the unique VINT-encoded tag of the element.
	Tag uint64
	// DataSize defines how many bytes of data the element contains.
	DataSize uint64
}

// Reader reads EBML elements.
type Reader struct {
	reader        io.Reader
	elementReader io.Reader
}

// NewReader creates a new Reader that will read from reader.
func NewReader(reader io.Reader) *Reader {
	return &Reader{
		reader: reader,
	}
}

// NextElement reads the next element's header.
func (r *Reader) NextElement() (uint64, uint64, error) {
	_, tag, err := ReadVINT(r.reader)
	if err != nil {
		return 0, 0, err
	}

	size, _, err := ReadVINT(r.reader)
	if err != nil {
		return 0, 0, err
	}
	r.elementReader = io.LimitReader(r.reader, int64(size))

	// For whate
	return tag, size, nil
}

// Reader returns the current element's data reader.
func (r *Reader) Reader() io.Reader {
	return r.elementReader
}

// Read reads from the current element's data.
func (r *Reader) Read(p []byte) (int, error) {
	return r.elementReader.Read(p)
}

// Discard discards the current element's data.
func (r *Reader) Discard() (int64, error) {
	if r.elementReader == nil {
		return 0, fmt.Errorf("ebml: element header not read")
	}

	return io.Copy(io.Discard, r.elementReader)
}

// ReadVINT reads a Variable-Size Integer.
// Returns the integer and VINT representation.
// SEE: https://github.com/ietf-wg-cellar/ebml-specification/blob/master/specification.markdown#variable-size-integer.
func ReadVINT(r io.Reader) (uint64, uint64, error) {
	var b [1]byte
	_, err := r.Read(b[:])
	if err != nil {
		return 0, 0, err
	}

	// NOTE: For sanity reasons, don't expect more than 8B integers
	if b[0] == 0 {
		return 0, 0, fmt.Errorf("ebml: vint is too long")
	}

	width := bits.LeadingZeros8(uint8(b[0])) + 1
	var integer uint64
	vint := uint64(b[0])
	if width < 7 {
		m := uint8(1<<(8-width)) - 1
		integer = uint64(b[0]) & uint64(m)
	}
	for j := 0; j < width-1; j++ {
		var b [1]byte
		_, err := r.Read(b[:])
		if err != nil {
			return 0, 0, err
		}

		integer = integer<<8 | uint64(b[0])
		vint = vint<<8 | uint64(b[0])
	}
	return integer, vint, nil
}
