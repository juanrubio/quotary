package tui

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/juanrubio/quotary/internal/backend/demo"
)

// This file contains the terminal-level regression test for the kitty REP
// compatibility defect (docs/evidence/kitty-rep-compatibility.md): kitty
// versions without REP support discard CSI Ps b, which collapses the compact
// overview table's column padding.
//
// The test renders the compact overview through a real Bubble Tea program
// with the renderer's kitty capability set active (TERM=xterm-kitty, which
// makes ultraviolet compress runs of repeated characters with REP), captures
// the byte stream, and interprets it on a simulated terminal that drops REP
// exactly like kitty 0.16.0 does. Running the emulator this way is what the
// golden captures cannot see: they assert the model's frame, not the byte
// stream a terminal interprets.

// vtScreen is a minimal screen emulator covering the sequence subset the
// Bubble Tea renderer emits: CUP, CUU/CUD/CUF/CUB, HPA/VPA, ECH, ICH, EL,
// ED, SU/SD, REP, SGR (ignored), private modes (ignored), OSC/DCS/APC
// payloads (ignored), and CR/LF/BS/TAB. Line feeds are treated as CR+LF,
// matching the newline mapping the renderer relies on when the input is not
// a terminal (which is how this test drives the program).
//
// repEnabled selects between a REP-capable terminal and one that parses and
// discards REP — the kitty 0.16.0 behaviour. repCount reports how many REP
// sequences were parsed, regardless of repEnabled.
type vtScreen struct {
	width, height  int
	cells          [][]rune
	x, y           int
	savedX, savedY int
	last           rune
	repEnabled     bool
	repCount       int

	state     int
	params    []byte
	stringOSC bool
}

const (
	vtGround = iota
	vtEsc
	vtEscInter
	vtCSI
	vtString
	vtStringESC
)

// emulatedScreenWidth and emulatedScreenHeight size the emulated terminal.
// The compact overview is verified at 100x30; a resize to a smaller frame
// renders within the same screen.
const (
	emulatedScreenWidth  = 100
	emulatedScreenHeight = 30
)

func newVtScreen(repEnabled bool) *vtScreen {
	s := &vtScreen{width: emulatedScreenWidth, height: emulatedScreenHeight, repEnabled: repEnabled}
	s.cells = make([][]rune, s.height)
	for i := range s.cells {
		s.cells[i] = blankRow(s.width)
	}
	return s
}

// blankRow returns a row of blanks. A cell the renderer never wrote shows
// as a blank space on a real terminal, not as NUL.
func blankRow(width int) []rune {
	row := make([]rune, width)
	for i := range row {
		row[i] = ' '
	}
	return row
}

func (s *vtScreen) feed(b []byte) {
	for i := 0; i < len(b); {
		c := b[i]
		switch s.state {
		case vtGround:
			switch {
			case c == '\x1b':
				s.state = vtEsc
				i++
			case c == '\r':
				s.x = 0
				i++
			case c == '\n':
				s.x = 0
				s.index()
				i++
			case c == '\b':
				if s.x > 0 {
					s.x--
				}
				i++
			case c == '\t':
				s.x = ((s.x / 8) + 1) * 8
				if s.x >= s.width {
					s.x = s.width - 1
				}
				i++
			case c < 0x20 || c == 0x7f:
				i++ // other control characters are ignored
			default:
				r, size := utf8.DecodeRune(b[i:])
				if r == utf8.RuneError && size == 1 {
					r, size = '?', 1
				}
				s.put(r)
				i += size
			}
		case vtEsc:
			i++
			switch {
			case c == '[':
				s.state = vtCSI
				s.params = s.params[:0]
			case c == ']' || c == 'P' || c == 'X' || c == '^' || c == '_':
				s.state = vtString
				s.stringOSC = c == ']'
			case c == '7':
				s.savedX, s.savedY = s.x, s.y
				s.state = vtGround
			case c == '8':
				s.x, s.y = s.savedX, s.savedY
				s.state = vtGround
			case c == 'D':
				s.index()
				s.state = vtGround
			case c == 'M':
				s.reverseIndex()
				s.state = vtGround
			case c == 'E':
				s.x = 0
				s.index()
				s.state = vtGround
			case c == 'c':
				s.x, s.y = 0, 0
				s.clearScreen()
				s.state = vtGround
			case c >= 0x20 && c <= 0x2f:
				s.state = vtEscInter
			default:
				s.state = vtGround // two-byte escape sequence
			}
		case vtEscInter:
			i++
			if c < 0x20 || c > 0x2f {
				s.state = vtGround
			}
		case vtCSI:
			i++
			// A control character inside a CSI sequence never occurs in the
			// renderer's output; if it did, it is ignored and the sequence kept.
			if c >= 0x40 && c <= 0x7e {
				s.state = vtGround
				s.dispatch(c)
			} else if c >= 0x20 && c <= 0x3f {
				s.params = append(s.params, c)
			}
		case vtString:
			i++
			switch c {
			case '\x07':
				if s.stringOSC {
					s.state = vtGround
				}
			case '\x1b':
				s.state = vtStringESC
			}
		case vtStringESC:
			i++
			if c == '\\' {
				s.state = vtGround
			} else {
				s.state = vtString
			}
		}
	}
}

func (s *vtScreen) put(r rune) {
	if r >= 0x20 && r != 0x7f {
		s.last = r
	}
	if s.y >= 0 && s.y < s.height && s.x >= 0 && s.x < s.width {
		s.cells[s.y][s.x] = r
	}
	s.x++
}

func (s *vtScreen) index() {
	if s.y < s.height-1 {
		s.y++
	} else {
		s.scrollUp(1)
	}
}

func (s *vtScreen) reverseIndex() {
	if s.y > 0 {
		s.y--
	} else {
		s.scrollDown(1)
	}
}

func (s *vtScreen) scrollUp(n int) {
	for row := 0; row+n <= s.height-1; row++ {
		s.cells[row] = s.cells[row+n]
	}
	for row := s.height - n; row < s.height; row++ {
		if row >= 0 {
			s.cells[row] = blankRow(s.width)
		}
	}
}

func (s *vtScreen) scrollDown(n int) {
	for row := s.height - 1; row-n >= 0; row-- {
		s.cells[row] = s.cells[row-n]
	}
	for row := 0; row < n && row < s.height; row++ {
		s.cells[row] = blankRow(s.width)
	}
}

func (s *vtScreen) clearScreen() {
	for row := range s.cells {
		s.cells[row] = blankRow(s.width)
	}
}

func (s *vtScreen) dispatch(final byte) {
	if hasMarker(s.params) {
		return // private or annotated sequence: modes and queries, ignored
	}
	switch final {
	case 'H', 'f':
		row, col := s.param2()
		s.x = clampHigh(col-1, s.width-1)
		s.y = clampHigh(row-1, s.height-1)
	case 'A':
		s.y = clampHigh(s.y-s.param1(), s.height-1)
	case 'B':
		s.y = clampHigh(s.y+s.param1(), s.height-1)
	case 'C':
		s.x = clampHigh(s.x+s.param1(), s.width-1)
	case 'D':
		s.x = clampHigh(s.x-s.param1(), s.width-1)
	case 'G':
		s.x = clampHigh(s.param1()-1, s.width-1)
	case '`':
		s.x = clampHigh(s.param1()-1, s.width-1) // HPA, alternative form
	case 'd':
		s.y = clampHigh(s.param1()-1, s.height-1)
	case 'a':
		s.x = clampHigh(s.x+s.param1(), s.width-1) // HPR
	case 'e':
		s.y = clampHigh(s.y+s.param1(), s.height-1) // VPR
	case 'X':
		for j := 0; j < s.param1() && s.x+j < s.width; j++ {
			s.cells[s.y][s.x+j] = ' '
		}
	case '@':
		n := s.param1()
		for col := s.width - 1; col >= s.x+n && col >= 0; col-- {
			s.cells[s.y][col] = s.cells[s.y][col-n]
		}
		for j := 0; j < n && s.x+j < s.width; j++ {
			s.cells[s.y][s.x+j] = ' '
		}
	case 'K':
		switch s.param0() {
		case 0:
			for col := s.x; col < s.width; col++ {
				s.cells[s.y][col] = ' '
			}
		case 1:
			for col := 0; col <= s.x && col < s.width; col++ {
				s.cells[s.y][col] = ' '
			}
		case 2:
			for col := 0; col < s.width; col++ {
				s.cells[s.y][col] = ' '
			}
		}
	case 'J':
		switch s.param0() {
		case 0:
			for col := s.x; col < s.width; col++ {
				s.cells[s.y][col] = ' '
			}
			for row := s.y + 1; row < s.height; row++ {
				s.cells[row] = blankRow(s.width)
			}
		case 1:
			for row := 0; row < s.y; row++ {
				s.cells[row] = blankRow(s.width)
			}
			for col := 0; col <= s.x && col < s.width; col++ {
				s.cells[s.y][col] = ' '
			}
		case 2, 3:
			s.clearScreen()
		}
	case 'S':
		s.scrollUp(s.param1())
	case 'T':
		s.scrollDown(s.param1())
	case 'b':
		s.repCount++
		if s.repEnabled && s.last != 0 {
			for j := 0; j < s.param1(); j++ {
				s.put(s.last)
			}
		}
	}
}

func hasMarker(params []byte) bool {
	for _, c := range params {
		if c != ';' && (c < '0' || c > '9') {
			return true
		}
	}
	return false
}

func (s *vtScreen) param1() int {
	return s.paramDefault(1)
}

// param0 is like param1 but for sequences whose default parameter is 0
// (EL and ED modes).
func (s *vtScreen) param0() int {
	return s.paramDefault(0)
}

func (s *vtScreen) paramDefault(def int) int {
	parts := strings.Split(string(s.params), ";")
	if len(parts) == 0 || parts[0] == "" {
		return def
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n < 1 {
		return def
	}
	return n
}

func (s *vtScreen) param2() (int, int) {
	parts := strings.Split(string(s.params), ";")
	get := func(i int) int {
		if i >= len(parts) || parts[i] == "" {
			return 1
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 1 {
			return 1
		}
		return n
	}
	return get(0), get(1)
}

func clampHigh(v, high int) int {
	if v < 0 {
		return 0
	}
	if v > high {
		return high
	}
	return v
}

// rows returns the screen contents with trailing blanks trimmed.
func (s *vtScreen) rows() []string {
	rows := make([]string, len(s.cells))
	for i, row := range s.cells {
		rows[i] = strings.TrimRight(string(row), " ")
	}
	return rows
}

// captureProgramOutput runs the demo model through a real Bubble Tea program
// with the renderer's kitty capability set active, and returns the byte
// stream the program wrote. When wrap is true the stream passes through
// repExpandWriter exactly as in production; TERM=xterm-kitty makes the
// renderer compress runs of repeated characters with REP. Each resize in
// resizes is sent after the previous frame has painted, so multi-frame
// streams are covered too.
func captureProgramOutput(t *testing.T, wrap bool, resizes ...tea.WindowSizeMsg) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	var out io.Writer = w
	if wrap {
		out = newRepExpandWriter(w)
	}
	var mu sync.Mutex
	var captured bytes.Buffer
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				mu.Lock()
				captured.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil {
				close(done)
				return
			}
		}
	}()

	m := demoModel(t, demo.ScenarioSubscription)
	p := tea.NewProgram(m,
		tea.WithInput(bytes.NewBuffer(nil)),
		tea.WithOutput(out),
		tea.WithoutSignals(),
		tea.WithWindowSize(100, 30),
		tea.WithEnvironment([]string{"TERM=xterm-kitty", "COLORTERM=truecolor", "TTY_FORCE=1"}),
	)
	finished := make(chan struct{})
	go func() {
		_, _ = p.Run()
		close(finished)
	}()
	waitForPaint := func() {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			painted := strings.Contains(captured.String(), "State")
			mu.Unlock()
			if painted {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("overview did not paint")
	}
	waitForPaint()
	for _, resize := range resizes {
		mu.Lock()
		before := captured.Len()
		mu.Unlock()
		p.Send(resize)
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			grown := captured.Len() > before+50
			mu.Unlock()
			if grown {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	p.Quit()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("program did not quit")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	mu.Lock()
	defer mu.Unlock()
	return captured.Bytes()
}

func TestCompactOverviewColumnsSurviveREPDroppingTerminal(t *testing.T) {
	raw := captureProgramOutput(t, false)
	fixed := captureProgramOutput(t, true)

	reference := newVtScreen(true) // REP-capable terminal (kitty 0.45.0)
	reference.feed(raw)
	if reference.repCount == 0 {
		t.Fatal("control stream contains no REP; the renderer's kitty capability set is not active, so this test cannot detect the defect")
	}

	// Anchor the emulator against the model's own rendered view, so a
	// mistake in the emulator cannot make the comparisons below vacuous.
	expectedModel := demoModel(t, demo.ScenarioSubscription)
	expectedModel.width, expectedModel.height = 100, 30
	expected := newVtScreen(true)
	expected.feed([]byte(strings.TrimRight(expectedModel.View().Content, "\n")))
	if !reflect.DeepEqual(expected.rows(), reference.rows()) {
		t.Fatalf("emulated screen does not match the model's rendered view:\ngot:\n%s\nwant:\n%s",
			strings.Join(reference.rows(), "\n"), strings.Join(expected.rows(), "\n"))
	}

	dropped := newVtScreen(false) // REP-dropping terminal (kitty 0.16.0)
	dropped.feed(raw)

	fixedCapable := newVtScreen(true)
	fixedCapable.feed(fixed)
	fixedDropped := newVtScreen(false)
	fixedDropped.feed(fixed)

	if fixedDropped.repCount != 0 {
		t.Fatalf("wrapped stream still contains %d REP sequences", fixedDropped.repCount)
	}

	// Without the wrapper, the REP-dropping terminal loses the column
	// padding: this is the defect, and it is what the regression test must
	// keep detecting.
	header := func(rows []string) string {
		for _, row := range rows {
			if strings.Contains(row, "Allowance Left") {
				return row
			}
		}
		t.Fatal("overview header row not found")
		return ""
	}
	collapsed := header(dropped.rows())
	if !strings.Contains(collapsed, "Allowance Left Resets State") {
		t.Fatalf("expected collapsed column padding without the wrapper, got %q", collapsed)
	}

	// With the wrapper, the REP-dropping terminal renders the same screen a
	// REP-capable terminal renders from the unwrapped stream.
	if !reflect.DeepEqual(reference.rows(), fixedDropped.rows()) {
		t.Fatalf("screen on REP-dropping terminal differs from reference:\ngot:\n%s\nwant:\n%s",
			strings.Join(fixedDropped.rows(), "\n"), strings.Join(reference.rows(), "\n"))
	}

	// The wrapper is transparent on REP-capable terminals.
	if !reflect.DeepEqual(reference.rows(), fixedCapable.rows()) {
		t.Fatalf("wrapper changed the screen on a REP-capable terminal:\ngot:\n%s\nwant:\n%s",
			strings.Join(fixedCapable.rows(), "\n"), strings.Join(reference.rows(), "\n"))
	}

	// The initial window size is picked up and truecolor survives the
	// wrapper: the frame occupies the full 100x30 area (the pager sits on
	// the last row) and the stream still carries truecolor SGR sequences.
	if pager := reference.rows()[29]; !strings.Contains(pager, "20 / 2") {
		t.Fatalf("initial window size not picked up: last row is %q", pager)
	}
	if !bytes.Contains(fixed, []byte("\x1b[38;2;")) {
		t.Fatal("wrapped stream lost truecolor SGR sequences")
	}
}

func TestWrapperHandlesResizeRepaints(t *testing.T) {
	// A resize triggers a second diff-rendered frame through the wrapper's
	// retained state; the result must be the same on both terminal models.
	fixed := captureProgramOutput(t, true, tea.WindowSizeMsg{Width: 60, Height: 20})
	if !bytes.Contains(fixed, []byte("QUOTARY")) {
		t.Fatal("resize frame missing")
	}
	capable := newVtScreen(true)
	capable.feed(fixed)
	dropped := newVtScreen(false)
	dropped.feed(fixed)
	if dropped.repCount != 0 {
		t.Fatalf("wrapped stream still contains %d REP sequences", dropped.repCount)
	}
	if !reflect.DeepEqual(capable.rows(), dropped.rows()) {
		t.Fatal("resized frame differs between REP-capable and REP-dropping terminals")
	}
	if !strings.Contains(strings.Join(capable.rows(), "\n"), "QUOTARY") {
		t.Fatal("resized frame content lost")
	}
}
