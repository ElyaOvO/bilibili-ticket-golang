package terminal

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfirmOnceRetriesAndPersistsMarker(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "data", ".verified")
	var output bytes.Buffer
	prompted, err := ConfirmOnce(ConfirmationOptions{
		MarkerPath:     marker,
		RequiredText:   "required phrase",
		Prompt:         "input: ",
		RetryMessage:   "retry: ",
		RewriteOnRetry: true,
		SuccessMessage: "success",
		Input:          strings.NewReader("wrong\nrequired phrase\n"),
		Output:         &output,
	})
	if err != nil {
		t.Fatalf("ConfirmOnce() error = %v", err)
	}
	if !prompted {
		t.Fatal("ConfirmOnce() prompted = false, want true")
	}
	if got := output.String(); got != "input: retry: success\n" {
		t.Fatalf("output = %q", got)
	}
	contents, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if string(contents) != "1" {
		t.Fatalf("marker contents = %q", contents)
	}
}

func TestConfirmOnceSkipsExistingMarker(t *testing.T) {
	marker := filepath.Join(t.TempDir(), ".verified")
	if err := os.WriteFile(marker, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	prompted, err := ConfirmOnce(ConfirmationOptions{
		MarkerPath:   marker,
		RequiredText: "required phrase",
		Input:        strings.NewReader(""),
		Output:       &output,
	})
	if err != nil {
		t.Fatalf("ConfirmOnce() error = %v", err)
	}
	if prompted {
		t.Fatal("ConfirmOnce() prompted = true, want false")
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want empty", output.String())
	}
}

func TestConfirmOnceReturnsEOFWithoutMarker(t *testing.T) {
	marker := filepath.Join(t.TempDir(), ".verified")
	prompted, err := ConfirmOnce(ConfirmationOptions{
		MarkerPath:   marker,
		RequiredText: "required phrase",
		Input:        strings.NewReader(""),
		Output:       io.Discard,
	})
	if !prompted {
		t.Fatal("ConfirmOnce() prompted = false, want true")
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("ConfirmOnce() error = %v, want io.EOF", err)
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("marker unexpectedly exists: %v", statErr)
	}
}

func TestRenderStyledText(t *testing.T) {
	message := "{{bold}}heading{{/}} {{#ffffff}}white{{/}} {{#000000|#facc15|bold}}input{{/}}"
	wantANSI := ansiBold + "heading" + ansiReset + " " +
		"\x1b[38;2;255;255;255mwhite" + ansiReset + " " +
		"\x1b[1;38;2;0;0;0;48;2;250;204;21minput" + ansiReset
	if got := renderStyledTextWithColorMode(message, colorTrue); got != wantANSI {
		t.Fatalf("renderStyledText(ANSI) = %q, want %q", got, wantANSI)
	}
	want256 := ansiBold + "heading" + ansiReset + " " +
		"\x1b[38;5;231mwhite" + ansiReset + " " +
		"\x1b[1;38;5;16;48;5;220minput" + ansiReset
	if got := renderStyledTextWithColorMode(message, colorANSI256); got != want256 {
		t.Fatalf("renderStyledText(256 color) = %q, want %q", got, want256)
	}
	want16 := ansiBold + "heading" + ansiReset + " " +
		"\x1b[97mwhite" + ansiReset + " " +
		"\x1b[1;30;103minput" + ansiReset
	if got := renderStyledTextWithColorMode(message, colorANSI16); got != want16 {
		t.Fatalf("renderStyledText(16 color) = %q, want %q", got, want16)
	}
	if got := renderStyledTextWithColorMode(message, colorNone); got != "heading white input" {
		t.Fatalf("renderStyledText(plain) = %q", got)
	}
}

func TestColorModeFromCount(t *testing.T) {
	tests := []struct {
		colors int
		want   terminalColorMode
	}{
		{colors: -1, want: colorNone},
		{colors: 0, want: colorNone},
		{colors: 8, want: colorANSI16},
		{colors: 16, want: colorANSI16},
		{colors: 256, want: colorANSI256},
		{colors: 1 << 24, want: colorTrue},
	}
	for _, test := range tests {
		if got := colorModeFromCount(test.colors); got != test.want {
			t.Errorf("colorModeFromCount(%d) = %d, want %d", test.colors, got, test.want)
		}
	}
}
