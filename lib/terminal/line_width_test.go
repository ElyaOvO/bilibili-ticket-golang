package terminal

import (
	"bytes"
	"testing"
)

func TestRuneDisplayWidth(t *testing.T) {
	tests := []struct {
		value rune
		want  int
	}{
		{value: 'A', want: 1},
		{value: '中', want: 2},
		{value: '。', want: 2},
		{value: '\u0301', want: 0},
	}
	for _, test := range tests {
		if got := runeDisplayWidth(test.value); got != test.want {
			t.Errorf("runeDisplayWidth(%q) = %d, want %d", test.value, got, test.want)
		}
	}
}

func TestEraseRune(t *testing.T) {
	var output bytes.Buffer
	if err := eraseRune(&output, '中'); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "\b\b  \b\b"; got != want {
		t.Fatalf("eraseRune() = %q, want %q", got, want)
	}
}
