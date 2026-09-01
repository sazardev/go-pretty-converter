package format

import "strings"

// fenceMarker is the fenced-code-block delimiter this package recognizes.
// Only the common triple-backtick form is handled — ~~~ fences are rare
// enough in raw prose that adding a second marker isn't worth the extra
// branching for this heuristic pass.
const fenceMarker = "```"

// block is one blank-line-delimited chunk of raw text, split fence-aware so
// a fenced code block containing an internal blank line stays one block
// instead of fragmenting — see splitBlankLineBlocks.
type block struct {
	lines []string
}

func (b block) text() string {
	return strings.Join(b.lines, "\n")
}

// splitBlankLineBlocks splits raw text into blank-line-delimited blocks,
// but treats any run of lines between a pair of ``` fence lines as a single
// unbreakable block regardless of blank lines inside it. A naive
// regexp-based blank-line split (the approach mdx/plaintext.go's
// renderPlainText uses for literal .txt rendering, where it's harmless)
// would otherwise fragment a pasted code listing that has a blank line
// between two functions into several separate blocks.
func splitBlankLineBlocks(raw string) []block {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")

	var blocks []block
	var cur []string
	inFence := false

	flush := func() {
		for len(cur) > 0 && strings.TrimSpace(cur[0]) == "" {
			cur = cur[1:]
		}
		for len(cur) > 0 && strings.TrimSpace(cur[len(cur)-1]) == "" {
			cur = cur[:len(cur)-1]
		}
		if len(cur) > 0 {
			blocks = append(blocks, block{lines: cur})
		}
		cur = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, fenceMarker) {
			inFence = !inFence
			cur = append(cur, line)
			continue
		}
		if !inFence && trimmed == "" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()

	return blocks
}

func isFencedBlock(b block) bool {
	if len(b.lines) < 2 {
		return false
	}
	first := strings.TrimSpace(b.lines[0])
	last := strings.TrimSpace(b.lines[len(b.lines)-1])
	return strings.HasPrefix(first, fenceMarker) && last == fenceMarker
}
