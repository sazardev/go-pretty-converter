package epub

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

// validateWellFormedXHTML is a hard check every chapter must pass: EPUB
// XHTML files are served as application/xhtml+xml, so a chapter that is
// not well-formed XML is rejected by strict readers and, worse, gets
// *silently "fixed"* by Calibre during Kindle conversion in ways that
// splice visible text (the mid-word fusions and merged sentences seen on
// Kindle are exactly this failure mode). Chapters are built through
// x/html's lenient parser and XHTML renderer, which should always produce
// well-formed output; this guard turns any slip into a build error with
// the offending chapter named, instead of a broken file shipped to the
// reader.
func validateWellFormedXHTML(xhtml string) error {
	dec := xml.NewDecoder(bytes.NewReader([]byte(xhtml)))
	for {
		if _, err := dec.Token(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("chapter XHTML is not well-formed XML: %w", err)
		}
	}
}
