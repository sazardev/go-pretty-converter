// Command fakeebookconvert stands in for Calibre's ebook-convert in the
// kindle package's tests, so those tests run without Calibre installed.
// Behavior is selected via the FAKE_EBOOK_CONVERT_MODE env var:
//   - "" or "success" (default): copies argv[1] (input) to argv[2] (output)
//   - "fail": prints to stderr and exits 1, writing no output
//   - "hang": sleeps far longer than any test timeout
package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	switch os.Getenv("FAKE_EBOOK_CONVERT_MODE") {
	case "fail":
		fmt.Fprintln(os.Stderr, "fake ebook-convert: simulated conversion failure")
		os.Exit(1)
	case "hang":
		time.Sleep(time.Hour)
		return
	}

	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "fake ebook-convert: expected <input> <output>")
		os.Exit(2)
	}

	in, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
