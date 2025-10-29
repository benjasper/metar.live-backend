package utils

import (
	"compress/gzip"
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
func DownloadFile(ctx context.Context, url string, filepath string, useGzip bool) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		_ = out.Close()
	}(out)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not download file from %s: %s", url, resp.Status)
	}

	var content io.Reader = resp.Body
	if useGzip {
		gzreader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}

		content = gzreader
		defer func() {
			_ = gzreader.Close()
		}()
	}

	// Writer the body to file
	_, err = io.Copy(out, content)
	if err != nil {
		return err
	}

	return nil
}

// RadiansToDegrees Converts Radians to Degrees
func RadiansToDegrees(radians float64) float64 {
	return radians * 180 / math.Pi
}

// DegreesToRadians Converts Degrees to Radians
func DegreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}
