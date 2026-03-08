package service

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	return img
}

func TestGetCompressor_ReturnsJPEGCompressorForJpeg(t *testing.T) {
	c, err := getCompressor("jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := c.(*JPEGCompressor); !ok {
		t.Errorf("expected *JPEGCompressor, got %T", c)
	}
}

func TestGetCompressor_ReturnsJPEGCompressorForJpg(t *testing.T) {
	c, err := getCompressor("jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := c.(*JPEGCompressor); !ok {
		t.Errorf("expected *JPEGCompressor, got %T", c)
	}
}

func TestGetCompressor_ReturnsPNGCompressorForPng(t *testing.T) {
	c, err := getCompressor("png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := c.(*PNGCompressor); !ok {
		t.Errorf("expected *PNGCompressor, got %T", c)
	}
}

func TestGetCompressor_ReturnsErrorForUnsupportedFormat(t *testing.T) {
	_, err := getCompressor("gif")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestJPEGCompress_ProducesValidJPEGOutput(t *testing.T) {
	c := &JPEGCompressor{}
	data, err := c.Compress(context.Background(), testImage())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty output")
	}
	_, err = jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("output is not valid JPEG: %v", err)
	}
}

func TestPNGCompress_ProducesValidPNGOutput(t *testing.T) {
	c := &PNGCompressor{}
	data, err := c.Compress(context.Background(), testImage())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty output")
	}
	_, err = png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("output is not valid PNG: %v", err)
	}
}

func TestJPEGCompressor_ContentTypeReturnsImageJpeg(t *testing.T) {
	c := &JPEGCompressor{}
	if got := c.ContentType(); got != "image/jpeg" {
		t.Errorf("expected 'image/jpeg', got %s", got)
	}
}

func TestPNGCompressor_ContentTypeReturnsImagePng(t *testing.T) {
	c := &PNGCompressor{}
	if got := c.ContentType(); got != "image/png" {
		t.Errorf("expected 'image/png', got %s", got)
	}
}
