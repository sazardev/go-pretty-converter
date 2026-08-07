package mdx

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type ComponentHandler func(attrs map[string]string, innerHTML string) string

// ComponentRegistry is safe for concurrent use: Register and Transpile may
// be called from multiple goroutines sharing the same Parser.
type ComponentRegistry struct {
	mu       sync.RWMutex
	handlers map[string]ComponentHandler
	usage    map[string]int
}

func NewComponentRegistry() *ComponentRegistry {
	r := &ComponentRegistry{
		handlers: make(map[string]ComponentHandler),
		usage:    make(map[string]int),
	}
	r.Register("DeepDive", deepDiveHandler)
	r.Register("Warning", warningHandler)
	r.Register("Axiom", axiomHandler)
	return r
}

func (r *ComponentRegistry) Register(name string, handler ComponentHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

// Usage reports how many times each registered component name matched in
// Transpile since the registry was created (or since the last ResetUsage).
// Callers use it to detect components that were registered via
// WithComponent() but never used in any document — a common authoring slip
// that wastes setup and may indicate a typo'd tag.
func (r *ComponentRegistry) Usage() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]int, len(r.usage))
	for name, n := range r.usage {
		out[name] = n
	}
	return out
}

// Names lists every registered component name (builtin and custom).
func (r *ComponentRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ResetUsage zeroes the usage counters (used between independent builds on
// the same Parser).
func (r *ComponentRegistry) ResetUsage() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usage = make(map[string]int)
}

// markUsed records that a component matched during Transpile.
func (r *ComponentRegistry) markUsed(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usage[name]++
}

// maxComponentNestingDepth bounds the innermost-first unwinding loop in
// Transpile (see below) so a pathological input can't spin it forever; any
// real document nests a handful of components at most.
const maxComponentNestingDepth = 64

func (r *ComponentRegistry) Transpile(input string) string {
	r.mu.RLock()
	handlers := make(map[string]ComponentHandler, len(r.handlers))
	for name, handler := range r.handlers {
		handlers[name] = handler
	}
	r.mu.RUnlock()

	result := input
	for name, handler := range handlers {
		re := buildComponentRegex(name)
		replace := func(match string) string {
			matches := re.FindStringSubmatch(match)
			attrs := parseComponentAttrs(matches[1])
			content := strings.TrimSpace(matches[2])
			r.markUsed(name)
			return handler(attrs, content)
		}
		// The non-greedy (.*?) below only ever matches a pair with no
		// nested tag of the *same* name inside it, so on input like
		// <Warning><Warning>x</Warning></Warning> a single pass matches
		// only the innermost pair and leaves the outer </Warning> as a
		// stray literal. Repeating the replacement until no match
		// remains unwinds one nesting level per pass instead.
		for i := 0; i < maxComponentNestingDepth && re.MatchString(result); i++ {
			result = re.ReplaceAllStringFunc(result, replace)
		}
	}
	return result
}

func buildComponentRegex(name string) *regexp.Regexp {
	pattern := fmt.Sprintf(`(?s)<%s(?:\s+([^>]*))?\s*>(.*?)</%s>`, name, name)
	return regexp.MustCompile(pattern)
}

// componentAttrRe matches a single name="value" or name='value' attribute
// pair, so component tags aren't limited to the exact literal
// `title="..."` shape — single-quoted values and any other attributes
// (ignored by the built-in handlers, but not fatal to parsing) all match.
// The value itself isn't captured: Go's regexp reports an unmatched group
// as "" indistinguishably from a group that matched an empty string, so
// there's no reliable way to tell which quote-style alternative fired from
// its submatches alone. parseComponentAttrs instead slices the value out of
// the whole match, which is unambiguous.
var componentAttrRe = regexp.MustCompile(`[A-Za-z_][\w-]*\s*=\s*(?:"[^"]*"|'[^']*')`)

func parseComponentAttrs(raw string) map[string]string {
	attrs := map[string]string{}
	for _, whole := range componentAttrRe.FindAllString(raw, -1) {
		eq := strings.IndexByte(whole, '=')
		name := strings.TrimSpace(whole[:eq])
		value := strings.TrimSpace(whole[eq+1:])
		if len(value) >= 2 {
			value = value[1 : len(value)-1] // strip the matching quote pair
		}
		attrs[name] = value
	}
	return attrs
}

func deepDiveHandler(attrs map[string]string, inner string) string {
	var buf strings.Builder
	buf.WriteString(`<aside class="component-deep-dive">`)
	if attrs["title"] != "" {
		fmt.Fprintf(&buf, `<div class="component-deep-dive-title">%s</div>`+"\n", html.EscapeString(attrs["title"]))
	}
	buf.WriteString(rewriteInlineContent(inner))
	buf.WriteString("</aside>")
	return buf.String()
}

func warningHandler(attrs map[string]string, inner string) string {
	var buf strings.Builder
	buf.WriteString(`<div class="component-warning">`)
	if attrs["title"] != "" {
		fmt.Fprintf(&buf, `<div class="component-warning-title">%s</div>`+"\n", html.EscapeString(attrs["title"]))
	}
	buf.WriteString(rewriteInlineContent(inner))
	buf.WriteString("</div>")
	return buf.String()
}

func axiomHandler(attrs map[string]string, inner string) string {
	var buf strings.Builder
	buf.WriteString(`<blockquote class="component-axiom">`)
	if attrs["title"] != "" {
		fmt.Fprintf(&buf, `<div class="component-axiom-title">%s</div>`+"\n", html.EscapeString(attrs["title"]))
	}
	buf.WriteString(rewriteInlineContent(inner))
	buf.WriteString("</blockquote>")
	return buf.String()
}

var codeSpanRe = regexp.MustCompile("`([^`]+)`")
var boldRe = regexp.MustCompile(`\*\*([^*]+)\*\*`)

func rewriteInlineContent(content string) string {
	result := codeSpanRe.ReplaceAllString(content, "<code>$1</code>")
	result = boldRe.ReplaceAllString(result, "<strong>$1</strong>")
	result = strings.ReplaceAll(result, "\n", "<br/>\n")
	return result
}
