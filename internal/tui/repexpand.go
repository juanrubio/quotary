package tui

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"sync"
	"unicode/utf8"
)

const esc = '\x1b'

// maxSequenceBytes bounds how much of an unterminated escape sequence the
// scanner buffers before giving up on it. Well-formed renderer output never
// comes close; the bound exists so a malformed stream cannot grow the buffer
// without limit.
const maxSequenceBytes = 1 << 20

// maxREPCount bounds how many repetitions a single REP sequence may expand
// to. The renderer only compresses runs of repeated characters within a
// line, so legitimate counts are far below this.
const maxREPCount = 1 << 16

// repScanState tracks where the incremental scanner is inside the byte
// stream. The states distinguish CSI sequences from OSC/DCS/APC string
// payloads, which must never be scanned for control sequences.
type repScanState int

const (
	repStateGround repScanState = iota
	repStateEscape
	repStateEscapeInter
	repStateCSI
	repStateOSC    // OSC payload: BEL or ST terminates
	repStateString // DCS/SOS/PM/APC payload: only ST terminates
	repStateStringESC
)

const bel = '\a'

// repExpandWriter expands REP escape sequences (CSI Ps b, ECMA-48 "REPEAT
// CHARACTER") into literal repetitions of the preceding graphic character.
//
// Terminals that do not implement REP — kitty 0.16.0 among them — silently
// discard it, so every run of repeated characters that the renderer compressed
// with REP is lost, which collapses the compact overview table's column
// padding. Expanding the sequence before it reaches the terminal is
// semantically transparent: a terminal that does implement REP receives the
// same characters it would have received anyway, spelled out. See
// docs/evidence/kitty-rep-compatibility.md.
//
// The wrapper is applied unconditionally because REP cannot be probed for:
// kitty advertises no rep capability in terminfo in any version, and a
// capability query for it does not exist.
//
// The embedded *os.File keeps the writer satisfying term.File (Fd, Read,
// Write, Close), which Bubble Tea and colour-profile detection type-assert
// the program output for. Losing that assertion would silently break window
// sizing, resize listening, terminal restoration, and truecolor detection, so
// the embedding is load-bearing.
//
// Write is safe for concurrent use. It never modifies its input and returns
// the length of the input, per the io.Writer contract.
type repExpandWriter struct {
	*os.File

	mu      sync.Mutex
	pending []byte // partial UTF-8 rune carried over from the previous Write
	seq     []byte // bytes of the escape sequence being scanned
	state   repScanState
	out     bytes.Buffer

	lastRun [utf8.UTFMax]byte // the most recent graphic character
	lastLen int               // 0 when no graphic character has been seen

	stringKind repScanState // which string state repStateStringESC returns to
}

// newRepExpandWriter wraps f so that REP sequences in the output are expanded
// into literal characters before they reach f.
func newRepExpandWriter(f *os.File) *repExpandWriter {
	return &repExpandWriter{File: f}
}

// The wrapper must keep satisfying the interface Bubble Tea type-asserts the
// program output for (charmbracelet/x/term.File). Embedding *os.File promotes
// Fd, Read, and Close; Write is provided above.
var _ interface {
	io.ReadWriteCloser
	Fd() uintptr
} = (*repExpandWriter)(nil)

// Write expands REP sequences in p and writes the result to the underlying
// file.
func (w *repExpandWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// A rune split across writes must be reassembled before it can be
	// decoded and counted as the preceding graphic character.
	w.pending = append(w.pending, p...)
	w.out.Reset()
	consumed := w.scan(&w.out, w.pending)
	w.pending = append(w.pending[:0], w.pending[consumed:]...)

	if w.out.Len() == 0 {
		return len(p), nil
	}
	n, err := w.File.Write(w.out.Bytes())
	if err != nil {
		return len(p), err
	}
	if n != w.out.Len() {
		return len(p), io.ErrShortWrite
	}
	return len(p), nil
}

// scan processes buf, writing the expanded bytes to out, and returns how much
// of buf was consumed. Ground-state bytes are emitted as they are scanned.
// The bytes of an escape sequence are held back in w.seq until the sequence
// is complete, because a REP sequence must not reach the terminal unless the
// wrapper decides to pass it through. An incomplete UTF-8 rune at the end of
// buf is not consumed; Write carries it over to the next call.
func (w *repExpandWriter) scan(out *bytes.Buffer, buf []byte) int {
	for i := 0; i < len(buf); {
		if w.state != repStateGround && len(w.seq) > maxSequenceBytes {
			// Malformed stream: flush the partial sequence unchanged and
			// resynchronize instead of buffering without bound.
			out.Write(w.seq)
			w.seq = w.seq[:0]
			w.state = repStateGround
		}
		b := buf[i]
		switch w.state {
		case repStateGround:
			if b == esc {
				w.state = repStateEscape
				w.seq = append(w.seq, esc)
				i++
			} else if b < 0x20 || b == 0x7f {
				// Control characters pass through and do not become the
				// preceding graphic character.
				out.WriteByte(b)
				i++
			} else {
				r, size := utf8.DecodeRune(buf[i:])
				if r == utf8.RuneError && size == 1 && !utf8.FullRune(buf[i:]) {
					return i // partial rune: carry over
				}
				out.Write(buf[i : i+size])
				if isGraphicRune(r) {
					w.setLast(r)
				}
				i += size
			}
		case repStateEscape:
			w.seq = append(w.seq, b)
			i++
			switch {
			case b == '[':
				w.state = repStateCSI
			case b == ']':
				w.state = repStateOSC
				w.stringKind = repStateOSC
			case b == 'P' || b == 'X' || b == '^' || b == '_':
				w.state = repStateString
				w.stringKind = repStateString
			case b >= 0x20 && b <= 0x2f:
				w.state = repStateEscapeInter
			default:
				// Final byte of a simple escape sequence, or a malformed
				// byte: the sequence ends here.
				w.flushSequence(out)
			}
		case repStateEscapeInter:
			w.seq = append(w.seq, b)
			i++
			if b < 0x20 || b > 0x2f {
				w.flushSequence(out)
			}
		case repStateCSI:
			if b < 0x20 || b == 0x7f {
				// A control character cannot be part of a CSI sequence. Flush
				// the partial sequence unchanged and reprocess the byte in
				// ground state.
				w.flushSequence(out)
				continue
			}
			w.seq = append(w.seq, b)
			i++
			if b >= 0x40 && b <= 0x7e {
				// Final byte: the CSI sequence is complete.
				w.state = repStateGround
				w.emitSequence(out)
				w.seq = w.seq[:0]
			}
		case repStateOSC:
			w.seq = append(w.seq, b)
			i++
			switch b {
			case bel:
				w.flushSequence(out)
			case esc:
				w.state = repStateStringESC
			}
		case repStateString:
			w.seq = append(w.seq, b)
			i++
			if b == esc {
				w.state = repStateStringESC
			}
		case repStateStringESC:
			w.seq = append(w.seq, b)
			i++
			if b == '\\' { // ST terminates the string sequence.
				w.flushSequence(out)
			} else {
				w.state = w.stringKind
			}
		}
	}
	return len(buf)
}

// flushSequence emits the buffered escape sequence unchanged and returns to
// ground state.
func (w *repExpandWriter) flushSequence(out *bytes.Buffer) {
	out.Write(w.seq)
	w.seq = w.seq[:0]
	w.state = repStateGround
}

// emitSequence handles a completed CSI sequence: bare REP sequences are
// expanded into literal repetitions of the preceding graphic character, and
// everything else is emitted unchanged. The caller resets w.seq and the state.
func (w *repExpandWriter) emitSequence(out *bytes.Buffer) {
	if !isBareREP(w.seq) {
		w.flushSequence(out)
		return
	}
	count := 1 // an omitted or zero parameter means 1, as on real terminals
	if params := w.seq[2 : len(w.seq)-1]; len(params) > 0 {
		parsed, err := strconv.Atoi(string(params))
		if err != nil {
			// Unparseable (overflow) parameter: leave the sequence to the
			// terminal rather than guess.
			w.flushSequence(out)
			return
		}
		if parsed > 0 {
			count = parsed
		}
	}
	if count > maxREPCount {
		// A count this large cannot come from the renderer, which only
		// compresses runs within a line; leave the sequence to the terminal
		// rather than expand it into megabytes.
		w.flushSequence(out)
		return
	}
	if w.lastLen == 0 {
		// No preceding graphic character has been seen; pass the sequence
		// through unchanged instead of inventing a character to repeat.
		w.flushSequence(out)
		return
	}
	for j := 0; j < count; j++ {
		out.Write(w.lastRun[:w.lastLen])
	}
	w.seq = w.seq[:0]
	w.state = repStateGround
}

// isBareREP reports whether seq is a complete CSI sequence of the form
// CSI Ps b with a single, purely decimal parameter. Sequences with a private
// or intermediate marker (for example CSI ? 25 b) and multi-parameter forms
// are not REP, and neither is anything that does not end in b.
func isBareREP(seq []byte) bool {
	if len(seq) < 3 || seq[0] != esc || seq[1] != '[' || seq[len(seq)-1] != 'b' {
		return false
	}
	for _, c := range seq[2 : len(seq)-1] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isGraphicRune(r rune) bool {
	switch {
	case r < 0x20 || r == 0x7f:
		return false // C0 controls and DEL
	case r >= 0x80 && r < 0xa0:
		return false // C1 controls
	default:
		return true
	}
}

func (w *repExpandWriter) setLast(r rune) {
	w.lastLen = utf8.EncodeRune(w.lastRun[:], r)
}
