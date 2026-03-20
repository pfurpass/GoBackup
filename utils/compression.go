package utils

import (
	"bytes"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

var (
	encoder *zstd.Encoder
	decoder *zstd.Decoder
)

func init() {
	var err error
	encoder, err = zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		panic("zstd encoder init: " + err.Error())
	}
	decoder, err = zstd.NewReader(nil)
	if err != nil {
		panic("zstd decoder init: " + err.Error())
	}
}

// CompressChunk compresses data using zstd and returns the compressed bytes.
func CompressChunk(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, fmt.Errorf("zstd writer: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("zstd write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("zstd flush: %w", err)
	}
	return buf.Bytes(), nil
}

// DecompressChunk decompresses zstd-compressed data.
func DecompressChunk(data []byte) ([]byte, error) {
	r, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("zstd reader: %w", err)
	}
	defer r.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("zstd decompress: %w", err)
	}
	return buf.Bytes(), nil
}
