package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
)

type result struct {
	data []byte
	err  error
}

type Compressor interface {
	Compress(ctx context.Context, img image.Image) ([]byte, error)
	ContentType() string
}

func getCompressor(format string) (Compressor, error) {
	if format == "jpg" || format == "jpeg" {
		return &JPEGCompressor{}, nil
	}

	if format == "png" {
		return &PNGCompressor{}, nil
	}

	return nil, fmt.Errorf("Unsupported file format: %s", format)
}

type JPEGCompressor struct{}

func (c *JPEGCompressor) Compress(ctx context.Context, img image.Image) ([]byte, error) {
	resCh := make(chan result, 1)

	go func() {
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 60})
		resCh <- result{data: buf.Bytes(), err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resCh:
		return res.data, res.err
	}

}

func (c *JPEGCompressor) ContentType() string {
	return "image/jpeg"
}

type PNGCompressor struct{}

func (c *PNGCompressor) Compress(ctx context.Context, img image.Image) ([]byte, error) {
	resCh := make(chan result, 1)

	go func() {
		var buf bytes.Buffer
		err := png.Encode(&buf, img)
		resCh <- result{data: buf.Bytes(), err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resCh:
		return res.data, res.err
	}
}

func (c *PNGCompressor) ContentType() string {
	return "image/png"
}
