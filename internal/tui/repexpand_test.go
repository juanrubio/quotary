package tui

import (
	"bytes"
	"os"
	"testing"
)

// capturePipe collects everything written through a repExpandWriter wrapping
// the write end of an os.Pipe, so the wrapper's *os.File embedding is
// exercised exactly as it is in production.
type capturePipe struct {
	w    *repExpandWriter
	done chan struct{}
	buf  bytes.Buffer
}

func newCapturePipe(t *testing.T) *capturePipe {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	c := &capturePipe{w: newRepExpandWriter(w), done: make(chan struct{})}
	go func() {
		_, _ = c.buf.ReadFrom(r)
		close(c.done)
	}()
	t.Cleanup(func() {
		_ = r.Close()
	})
	return c
}

func (c *capturePipe) write(t *testing.T, p []byte) {
	t.Helper()
	snapshot := append([]byte(nil), p...)
	n, err := c.w.Write(p)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != len(p) {
		t.Fatalf("write returned %d, want %d", n, len(p))
	}
	if !bytes.Equal(p, snapshot) {
		t.Fatalf("write modified its input: %q became %q", snapshot, p)
	}
}

func (c *capturePipe) finish(t *testing.T) string {
	t.Helper()
	if err := c.w.Close(); err != nil {
		t.Fatal(err)
	}
	<-c.done
	return c.buf.String()
}

func writeAllAndFinish(t *testing.T, chunks ...[]byte) string {
	t.Helper()
	c := newCapturePipe(t)
	for _, chunk := range chunks {
		c.write(t, chunk)
	}
	return c.finish(t)
}

func TestRepExpandWriterExpandsREP(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"five spaces after a space", " \x1b[5b|", "      |"},
		{"character plus repeats", "X\x1b[4bY", "XXXXXY"},
		{"omitted parameter means one", "X\x1b[bY", "XXY"},
		{"zero parameter means one", "X\x1b[0bY", "XXY"},
		{"multi-byte graphic character", "é\x1b[2b.", "ééé."},
		{"box drawing character", "─\x1b[3b+", "────+"},
		{"sgr between character and rep keeps it", "X\x1b[0m\x1b[3bY", "X\x1b[0mXXXY"},
		{"control character between character and rep keeps it", "X\n\x1b[3bY", "X\nXXXY"},
		{"escape sequences between character and rep keep it", "X\x1b7\x1b8\x1b[2bY", "X\x1b7\x1b8XXY"},
		{"bel-terminated osc keeps character for later rep", "X\x1b]2;ok\a\x1b[3bY", "X\x1b]2;ok\aXXXY"},
		{"last rep uses character before earlier ones", "A\x1b[2bB\x1b[2bC", "AAABBBC"},
		{"expansion participates in later runs", "A\x1b[2b\x1b[2bZ", "AAAAAZ"},
		{"rep after ech keeps character", "AB\x1b[2X\x1b[2bY", "AB\x1b[2XBBY"},
		{"rep after cursor positioning keeps character", "A\x1b[10G\x1b[3bY", "A\x1b[10GAAAY"},
		{"rep after charset escape keeps character", "X\x1b(B\x1b[2bY", "X\x1b(BXXY"},
		{"rep after st-terminated osc keeps character", "L\x1b]8;;u\x1b\\\x1b[2bY", "L\x1b]8;;u\x1b\\LLY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := writeAllAndFinish(t, []byte(tc.in)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRepExpandWriterPassesThroughNonREP(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"no escape sequences at all", "hello, world\n"},
		{"sgr only", "\x1b[38;2;10;20;30mtext\x1b[0m"},
		{"cursor moves", "\x1b[2;5H\x1b[16X\x1b[11;25Hx"},
		{"rep first in stream is not expanded", "\x1b[5bX"},
		{"control first in stream is not a character", "\n\x1b[5bX"},
		{"private marker is not rep", "X\x1b[?5bY"},
		{"multi parameter is not rep", "X\x1b[1;2bY"},
		{"intermediate byte is not rep", "X\x1b[5 bY"},
		{"other final byte is not rep", "X\x1b[5m\x1b[5G\x1b[5d\x1b[5X\x1b[5@Y"},
		{"rep inside osc payload is not touched", "X\x1b]0;title \x1b[5b\aY"},
		{"rep inside osc with st terminator", "X\x1b]8;;http://example.com\x1b[5b\x1b\\Y"},
		{"rep inside dcs payload is not touched", "X\x1bP1;2q\x1b[5b\x1b\\Y"},
		{"rep inside apc payload is not touched", "X\x1b_\x1b[5b\x1b\\Y"},
		{"bel does not terminate a dcs payload", "X\x1bPa\a\x1b[3b\x1b\\Y"},
		{"simple escape sequences pass through", "X\x1b(B\x1b7\x1b8Y"},
		{"control character mid csi flushes it", "X\x1b[12\nbY"},
		{"implausibly large count is left to the terminal", "X\x1b[999999bY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := writeAllAndFinish(t, []byte(tc.in)); got != tc.in {
				t.Fatalf("got %q, want identity %q", got, tc.in)
			}
		})
	}
}

func TestRepExpandWriterSplitWrites(t *testing.T) {
	cases := []struct {
		name   string
		chunks []string
		want   string
	}{
		{"rep split across writes", []string{"X", "\x1b[", "5", "b", "Y"}, "XXXXXXY"},
		{"rep parameter split across writes", []string{"X", "\x1b[1", "2b", "Y"}, "XXXXXXXXXXXXXY"},
		{"character in earlier write", []string{"X", "\x1b[3bY"}, "XXXXY"},
		{"write ends after escape", []string{"X", "\x1b", "[3bY"}, "XXXXY"},
		{"write ends after csi introducer", []string{"X", "\x1b[", "3bY"}, "XXXXY"},
		{"utf-8 character split across writes", []string{"\xc3", "\xa9\x1b[2b."}, "ééé."},
		{"sgr split across writes", []string{"X", "\x1b[0", "m\x1b[3b", "Y"}, "X\x1b[0mXXXY"},
		{"osc split across writes keeps payload rep", []string{"X", "\x1b]0;tit", "le \x1b[5b\a", "Y"}, "X\x1b]0;title \x1b[5b\aY"},
		{"rep after a split osc expands", []string{"X\x1b]0;a", "\x1b[5b\a\x1b[3b", "Y"}, "X\x1b]0;a\x1b[5b\aXXXY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := make([][]byte, len(tc.chunks))
			for i, s := range tc.chunks {
				chunks[i] = []byte(s)
			}
			if got := writeAllAndFinish(t, chunks...); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRepExpandWriterRoundTripIdentity(t *testing.T) {
	// A frame's worth of representative output containing everything the
	// renderer emits except REP: the wrapper must be byte-for-byte identity.
	frame := "\x1b[?25l\x1b[?1049h\x1b[H\x1b[2J" +
		"\x1b[38;2;255;255;255m  Source                \x1b[m" +
		"\x1b[1;30H\x1b[38;2;120;120;120mAllowance Left \x1b[m" +
		"\x1b[1;50H\x1b[38;2;120;120;120mResets \x1b[m" +
		"\x1b[1;60HState\r\n" +
		"> Claude Code\x1b[16X \x1b[11;25H5h  2h 14m" +
		"\x1b]0;quotary demo\a\x1b[?25h\x1b[?1049l"
	if got := writeAllAndFinish(t, []byte(frame)); got != frame {
		t.Fatalf("round-trip not identity:\ngot  %q\nwant %q", got, frame)
	}
}

func TestRepExpandWriterEmptyWrite(t *testing.T) {
	if got := writeAllAndFinish(t, []byte("X"), []byte{}, []byte("\x1b[2bY")); got != "XXXY" {
		t.Fatalf("got %q, want %q", got, "XXXY")
	}
}
