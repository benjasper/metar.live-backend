package utils

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
)

// Nillable returns the default value if the given value is nil.
func Nillable[T any](value *T) T {
	if value == nil {
		var val T
		return val
	}

	return *value
}

// NillableWithInput returns a pointer to the given value or nil if it's input string is empty.
func NillableWithInput[T any](input string, element T) *T {
	if input == "" {
		return nil
	}

	return &element
}

// NillableString returns a pointer to the given string or nil if it's input string is empty.
func NillableString(input string) *string {
	if input == "" {
		return nil
	}

	return &input
}

// DownloadFile downloads a file and saves it to disk using the provided context.
func DownloadFile(ctx context.Context, url string, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		_ = out.Close()
	}(out)

	body, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(body)

	_, err = io.Copy(out, body)
	return err
}

// Download downloads a file into memory using the provided context.
func Download(ctx context.Context, url string) ([]byte, error) {
	body, err := get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(body)

	return io.ReadAll(body)
}

func get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("could not download file from %s: %s", url, resp.Status)
	}

	return resp.Body, nil
}

// RadiansToDegrees Converts Radians to Degrees
func RadiansToDegrees(radians float64) float64 {
	return radians * 180 / math.Pi
}

// DegreesToRadians Converts Degrees to Radians
func DegreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}
