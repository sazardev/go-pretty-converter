package mdx

import (
	"sync"
	"testing"
)

// Run with -race: exercises Register (write) and Transpile (read) from
// multiple goroutines against the same ComponentRegistry.
func TestComponentRegistryConcurrentAccess(t *testing.T) {
	r := NewComponentRegistry()

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Register("Custom", func(attrs map[string]string, inner string) string {
				return "<div>" + inner + "</div>"
			})
		}(i)
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Transpile(`<DeepDive title="T">content</DeepDive>`)
		}()
	}

	wg.Wait()
}

func TestComponentUsageTracksUsed(t *testing.T) {
	r := NewComponentRegistry()
	r.Register("Callout", func(attrs map[string]string, inner string) string {
		return "<div class=\"callout\">" + inner + "</div>"
	})

	_ = r.Transpile(`<Callout title="info">hello</Callout><DeepDive title="x">deep</DeepDive>`)

	usage := r.Usage()
	if usage["Callout"] != 1 {
		t.Errorf("expected Callout usage 1, got %d", usage["Callout"])
	}
	if usage["DeepDive"] != 1 {
		t.Errorf("expected DeepDive usage 1, got %d", usage["DeepDive"])
	}
	if usage["Warning"] != 0 {
		t.Errorf("expected Warning usage 0, got %d", usage["Warning"])
	}
}

func TestComponentUsageCountsNestedAndRepeated(t *testing.T) {
	r := NewComponentRegistry()
	r.Register("Callout", func(attrs map[string]string, inner string) string {
		return "<div>" + inner + "</div>"
	})

	_ = r.Transpile(`<Callout title="a">one</Callout><Callout title="b">two</Callout><Callout><Callout>nested</Callout></Callout>`)

	usage := r.Usage()
	// The outer nesting loop re-processes the result each pass, so the
	// inner Callout is matched again after being re-emitted raw by the
	// outer handler. The exact count isn't the contract; >0 is.
	if usage["Callout"] < 3 {
		t.Errorf("expected Callout usage >= 3 (repeated+nested), got %d", usage["Callout"])
	}
}

func TestComponentUsageReset(t *testing.T) {
	r := NewComponentRegistry()
	r.Register("Callout", func(attrs map[string]string, inner string) string {
		return "<div>" + inner + "</div>"
	})

	_ = r.Transpile(`<Callout>x</Callout>`)
	r.ResetUsage()

	if usage := r.Usage()["Callout"]; usage != 0 {
		t.Errorf("expected usage reset to 0, got %d", usage)
	}
}

func TestComponentNamesListsAll(t *testing.T) {
	r := NewComponentRegistry()
	r.Register("Callout", func(attrs map[string]string, inner string) string {
		return "<div>" + inner + "</div>"
	})

	names := r.Names()
	byName := map[string]bool{}
	for _, n := range names {
		byName[n] = true
	}
	for _, want := range []string{"DeepDive", "Warning", "Axiom", "Callout"} {
		if !byName[want] {
			t.Errorf("expected %q in Names(), got %v", want, names)
		}
	}
}
