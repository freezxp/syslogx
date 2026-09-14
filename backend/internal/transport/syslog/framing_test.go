package syslog

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

func TestReadFrame(t *testing.T) {
	for _, tc := range []struct{ name, framing, input, want string }{
		{"newline", "newline", "<34>message\n", "<34>message"},
		{"octet", "octet_counting", "11 <34>message", "<34>message"},
		{"auto newline", "auto", "<34>message\n", "<34>message"},
		{"auto octet", "auto", "11 <34>message", "<34>message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readFrame(bufio.NewReader(strings.NewReader(tc.input)), tc.framing, 1024)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestReadFrameLimit(t *testing.T) {
	_, err := readFrame(bufio.NewReader(strings.NewReader("5 abcde")), "octet_counting", 4)
	if !errors.Is(err, errFrameTooLarge) {
		t.Fatalf("expected size error, got %v", err)
	}
}
