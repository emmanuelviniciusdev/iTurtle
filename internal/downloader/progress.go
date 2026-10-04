package downloader

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ProgressPrinter handles turtle-themed progress output
type ProgressPrinter struct {
	writer     io.Writer
	turtlePos  int
	lastUpdate time.Time
	turtles    []string
	mu         sync.Mutex
}

// NewProgressPrinter creates a new turtle progress printer
func NewProgressPrinter(w io.Writer) *ProgressPrinter {
	return &ProgressPrinter{
		writer:     w,
		turtlePos:  0,
		lastUpdate: time.Now(),
		turtles: []string{
			"🐢",
			"🐢",
			"🐢",
			"🐢",
		},
	}
}

// PrintStart prints the start of a download operation.
// It stays on the same line so the walking turtle can overwrite it.
func (p *ProgressPrinter) PrintStart(operation string) {
	fmt.Fprintf(p.writer, "\n🐢 %s...", operation)
}

// PrintProgress prints an animated progress indicator
func (p *ProgressPrinter) PrintProgress(message string) {
	// Only update animation every 200ms to avoid flickering
	if time.Since(p.lastUpdate) < 200*time.Millisecond {
		return
	}
	p.lastUpdate = time.Now()
	p.printFrame(message)
}

func (p *ProgressPrinter) printFrame(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.turtlePos = (p.turtlePos + 1) % 4

	var animation string
	switch p.turtlePos {
	case 0:
		animation = "🐢    "
	case 1:
		animation = " 🐢   "
	case 2:
		animation = "  🐢  "
	case 3:
		animation = "   🐢 "
	}

	message = truncateRunes(message, 60)
	fmt.Fprintf(p.writer, "\r%s %s\033[K", animation, message)
}

// startLive animates the turtle until Stop is called. SetMessage updates the
// text shown next to the turtle without interrupting the walk cycle.
func (p *ProgressPrinter) startLive(message string) *liveAnim {
	a := &liveAnim{
		p:       p,
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	a.print()
	go a.loop()
	return a
}

type liveAnim struct {
	p       *ProgressPrinter
	mu      sync.Mutex
	message string
	stop    chan struct{}
	done    chan struct{}
}

func (a *liveAnim) loop() {
	defer close(a.done)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-ticker.C:
			a.print()
		}
	}
}

func (a *liveAnim) print() {
	a.mu.Lock()
	msg := a.message
	a.mu.Unlock()
	a.p.printFrame(msg)
}

func (a *liveAnim) SetMessage(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	a.mu.Lock()
	a.message = message
	a.mu.Unlock()
}

func (a *liveAnim) Stop() {
	close(a.stop)
	<-a.done
	a.p.ClearLine()
}

// PrintComplete prints a completion message
func (p *ProgressPrinter) PrintComplete(message string, count int) {
	if count == 1 {
		fmt.Fprintf(p.writer, "\n✅ %s (1 file)\n", message)
	} else {
		fmt.Fprintf(p.writer, "\n✅ %s (%d files)\n", message, count)
	}
}

// PrintFile prints a completed file with turtle
func (p *ProgressPrinter) PrintFile(filename string) {
	// Truncate long filenames
	display := filename
	if len(display) > 60 {
		display = display[:57] + "..."
	}
	fmt.Fprintf(p.writer, "   🐢 %s\n", display)
}

// PrintError prints an error message
func (p *ProgressPrinter) PrintError(message string) {
	fmt.Fprintf(p.writer, "\n❌ %s\n", message)
}

// PrintWarning prints a warning message
func (p *ProgressPrinter) PrintWarning(message string) {
	fmt.Fprintf(p.writer, "\n⚠️  %s\n", message)
}

// PrintSection prints a section header
func (p *ProgressPrinter) PrintSection(title string) {
	border := strings.Repeat("─", len(title)+4)
	fmt.Fprintf(p.writer, "\n┌%s┐\n│  %s  │\n└%s┘\n", border, title, border)
}

// ClearLine clears the current line
func (p *ProgressPrinter) ClearLine() {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.writer, "\r\033[K")
}

func truncateRunes(s string, max int) string {
	if max < 4 {
		max = 4
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-3]) + "..."
}
