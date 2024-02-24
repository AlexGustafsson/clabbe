package ebml

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadVINT(t *testing.T) {
	testCases := []struct {
		Integer uint64
		VINT    uint64
		Bytes   []byte
	}{
		{
			Integer: 0x02,
			VINT:    0x82,
			Bytes:   []byte{0x82},
		},
		{
			Integer: 0x02,
			VINT:    0x4002,
			Bytes:   []byte{0x40, 0x02},
		},
		{
			Integer: 0x02,
			VINT:    0x200002,
			Bytes:   []byte{0x20, 0x00, 0x02},
		},
		{
			Integer: 0x02,
			VINT:    0x10000002,
			Bytes:   []byte{0x10, 0x00, 0x00, 0x02},
		},
		{
			Integer: 0x0a45dfa3,
			VINT:    0x1a45dfa3,
			Bytes:   []byte{0x1a, 0x45, 0xdf, 0xa3},
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("0x%x", testCase.VINT), func(t *testing.T) {
			r := bufio.NewReader(bytes.NewReader(testCase.Bytes))
			integer, vint, err := ReadVINT(r)
			assert.NoError(t, err)
			assert.Equal(t, testCase.Integer, integer)
			assert.Equal(t, testCase.VINT, vint)
			assert.Equal(t, 0, r.Buffered())
		})
	}
}

func TestRead(t *testing.T) {
	element := []byte{0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x86, 0x81, 0x01}

	r := bufio.NewReader(bytes.NewReader(element))
	reader := NewReader(r)

	tag, size, err := reader.NextElement()
	require.NoError(t, err)
	assert.Equal(t, uint64(0x1a45dfA3), tag)
	assert.Equal(t, uint64(0x1f), size)

	var content [4]byte
	n, err := reader.Read(content[:])
	require.NoError(t, err)
	assert.Equal(t, 4, n)

	assert.Equal(t, []byte{0x42, 0x86, 0x81, 0x01}, content[:])
}
