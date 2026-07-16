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
	if got := renderStyledText(message, true); got != wantANSI {
		t.Fatalf("renderStyledText(ANSI) = %q, want %q", got, wantANSI)
	}
	if got := renderStyledText(message, false); got != "heading white input" {
		t.Fatalf("renderStyledText(plain) = %q", got)
	}
}
