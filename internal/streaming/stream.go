package streaming

import (
	"io"
)

type AudioStream interface {
	io.ReadCloser
	Size() int64
	Title() string
	MimeType() string
}
