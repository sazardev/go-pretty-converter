package kindle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sazardev/go-pretty-converter/mdx"
)

func TestVerifyConvertedClean(t *testing.T) {
	t.Setenv("FAKE_EBOOK_CONVERT_MODE", "success")

	dir := t.TempDir()
	docs := []*mdx.Document{
		mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Introduction", "Hello."),
		mustParseDoc(t, dir, "b.mdx", "[2.0.0]", "Clean Architecture for Go APIs", "Imports point inward."),
	}

	outPath := filepath.Join(dir, "book.azw3")
	// The fake ebook-convert copies its input to its output, so the
	// "Kindle file" doubles as the extracted text layer.
	text := "Introduction\n\nEvery chapter so far.\n\nClean Architecture for Go APIs\n\nThe answer is discipline.\n"
	if err := os.WriteFile(outPath, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}

	audit, err := VerifyConverted(context.Background(), fakeEbookConvertPath, outPath, docs, 0)
	if err != nil {
		t.Fatalf("VerifyConverted: %v", err)
	}
	if !audit.OK() {
		t.Errorf("expected a clean audit, got findings: %+v", audit.Findings)
	}
}

func TestVerifyConvertedCatchesMarkupLeak(t *testing.T) {
	t.Setenv("FAKE_EBOOK_CONVERT_MODE", "success")

	dir := t.TempDir()
	docs := []*mdx.Document{mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Chapter One", "Hello.")}

	outPath := filepath.Join(dir, "book.azw3")
	// The exact leak a broken KF8 renderer shows: raw span markup with
	// chroma's token class and Calibre's aid anchor.
	text := "package main\nclass=\"chroma-w\" aid=\"4GULHE\">\nimport \"errors\"\n"
	if err := os.WriteFile(outPath, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}

	audit, err := VerifyConverted(context.Background(), fakeEbookConvertPath, outPath, docs, 0)
	if err != nil {
		t.Fatalf("VerifyConverted: %v", err)
	}
	if audit.OK() {
		t.Fatal("expected the markup leak to fail verification")
	}
	if !strings.Contains(audit.Findings[0].Message, "markup leaked") {
		t.Errorf("expected a markup-leak finding, got: %+v", audit.Findings)
	}
}

func TestVerifyConvertedCatchesMissingChapter(t *testing.T) {
	t.Setenv("FAKE_EBOOK_CONVERT_MODE", "success")

	dir := t.TempDir()
	docs := []*mdx.Document{
		mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Surviving Chapter", "Fine."),
		mustParseDoc(t, dir, "b.mdx", "[2.0.0]", "Mangled Chapter", "Content here."),
	}

	outPath := filepath.Join(dir, "book.azw3")
	// The second chapter's title never makes it into the extracted text,
	// as when a conversion swallows a heading.
	text := "Surviving Chapter\n\nFine text.\n"
	if err := os.WriteFile(outPath, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}

	audit, err := VerifyConverted(context.Background(), fakeEbookConvertPath, outPath, docs, 0)
	if err != nil {
		t.Fatalf("VerifyConverted: %v", err)
	}
	if audit.OK() {
		t.Fatal("expected the missing chapter to fail verification")
	}
	if !strings.Contains(audit.Findings[0].Message, "[2.0.0]") {
		t.Errorf("expected the finding to name the [2.0.0] chapter, got: %+v", audit.Findings)
	}
}

func TestVerifyConvertedModeFail(t *testing.T) {
	t.Setenv("FAKE_EBOOK_CONVERT_MODE", "fail")

	dir := t.TempDir()
	docs := []*mdx.Document{mustParseDoc(t, dir, "a.mdx", "[1.0.0]", "Chapter One", "Hello.")}
	outPath := filepath.Join(dir, "book.azw3")
	if err := os.WriteFile(outPath, []byte("whatever"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := VerifyConverted(context.Background(), fakeEbookConvertPath, outPath, docs, 0)
	if err == nil {
		t.Fatal("expected an error when text extraction fails")
	}
}

func TestAuditScanTextCaseInsensitiveTitles(t *testing.T) {
	// Titles are matched case- and whitespace-insensitively so reflow
	// and case differences don't cause false alarms.
	audit := &Audit{}
	docs := []*mdx.Document{mustParseDoc(t, t.TempDir(), "a.mdx", "[1.0.0]", "Clean Architecture for Go APIs", "x")}
	auditScanText(audit, "Clean\nArchitecture   for go apis\n", docs)
	if !audit.OK() {
		t.Errorf("expected normalized title match, got: %+v", audit.Findings)
	}
}
