package format

import (
	"reflect"
	"testing"
)

func TestSplitBlankLineBlocks(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []block
	}{
		{
			name: "two paragraphs",
			raw:  "First paragraph.\n\nSecond paragraph.",
			want: []block{
				{lines: []string{"First paragraph."}},
				{lines: []string{"Second paragraph."}},
			},
		},
		{
			name: "collapses multiple blank lines",
			raw:  "First.\n\n\n\nSecond.",
			want: []block{
				{lines: []string{"First."}},
				{lines: []string{"Second."}},
			},
		},
		{
			name: "trims leading and trailing blank lines",
			raw:  "\n\nHello.\n\n",
			want: []block{
				{lines: []string{"Hello."}},
			},
		},
		{
			name: "fenced block with internal blank line stays one block",
			raw:  fenceMarker + "\nfunc a() {}\n\nfunc b() {}\n" + fenceMarker,
			want: []block{
				{lines: []string{fenceMarker, "func a() {}", "", "func b() {}", fenceMarker}},
			},
		},
		{
			name: "fenced block followed by a paragraph",
			raw:  fenceMarker + "\ncode\n" + fenceMarker + "\n\nAfter the fence.",
			want: []block{
				{lines: []string{fenceMarker, "code", fenceMarker}},
				{lines: []string{"After the fence."}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitBlankLineBlocks(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitBlankLineBlocks(%q) =\n%#v\nwant\n%#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestIsFencedBlock(t *testing.T) {
	if !isFencedBlock(block{lines: []string{fenceMarker, "code", fenceMarker}}) {
		t.Error("expected a well-formed fenced block to be detected")
	}
	if isFencedBlock(block{lines: []string{"not fenced"}}) {
		t.Error("did not expect a plain line to be detected as fenced")
	}
}
