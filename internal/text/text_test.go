package text

import (
	"reflect"
	"strings"
	"testing"
)

func TestWrapStartsANewLineAtANewline(t *testing.T) {
	got := Sans.Wrap("Short one.\nSecond line here.", 15, 1000, 2)
	want := []string{"Short one.", "Second line here."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Wrap = %q, want %q", got, want)
	}
}

func TestWrapKeepsEveryLineWithinTheWidth(t *testing.T) {
	s := strings.Repeat("A long description that has to wrap. ", 3)
	lines := Sans.Wrap(s, 12, 200, 10)
	if len(lines) < 2 {
		t.Fatalf("Wrap = %q, want several lines", lines)
	}
	for _, l := range lines {
		if w := Sans.Width(l, 12, 0); w > 200 {
			t.Errorf("line %q is %.1fpx wide, want at most 200", l, w)
		}
	}
	if got, want := strings.Join(lines, " "), strings.Join(strings.Fields(s), " "); got != want {
		t.Errorf("lines join to %q, want the original words %q", got, want)
	}
}

func TestWrapCutsWhatDoesNotFit(t *testing.T) {
	s := strings.Repeat("A long description that has to wrap. ", 3)
	lines := Sans.Wrap(s, 12, 200, 2)
	if len(lines) != 2 {
		t.Fatalf("Wrap = %q, want 2 lines", lines)
	}
	if !strings.HasSuffix(lines[1], "…") {
		t.Errorf("last line %q does not end with an ellipsis", lines[1])
	}
	if w := Sans.Width(lines[1], 12, 0); w > 200 {
		t.Errorf("last line is %.1fpx wide, want at most 200", w)
	}
}

func TestWrapJoinsExtraLinesOntoTheLast(t *testing.T) {
	got := Sans.Wrap("one\ntwo\nthree", 15, 1000, 2)
	want := []string{"one", "two three"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Wrap = %q, want %q", got, want)
	}
}
