package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Indicator represents a progress indicator interface
type Indicator interface {
	Start(message string)
	Update(message string)
	Complete(message string)
	Fail(message string)
	Stop()
}

// Spinner creates a spinning progress indicator
type Spinner struct {
	writer   io.Writer
	message  string
	active   bool
	frames   []string
	interval time.Duration
	stopCh   chan bool
}

// NewSpinner creates a new spinner progress indicator
func NewSpinner() *Spinner {
	return &Spinner{
		writer:   os.Stdout,
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		interval: 80 * time.Millisecond,
		stopCh:   make(chan bool, 1),
	}
}

// Start begins the spinner with a message
func (s *Spinner) Start(message string) {
	s.message = message
	s.active = true
	
	go func() {
		i := 0
		lastMessage := ""
		for {
			select {
			case <-s.stopCh:
				return
			default:
				if s.active {
					currentFrame := fmt.Sprintf("%s %s", s.frames[i%len(s.frames)], s.message)
					if s.message != lastMessage {
						// Print new line for new messages
						fmt.Fprintf(s.writer, "\n%s", currentFrame)
						lastMessage = s.message
					} else {
						// Update in place for same message
						fmt.Fprintf(s.writer, "\r%s", currentFrame)
					}
					i++
					time.Sleep(s.interval)
				}
			}
		}
	}()
}

// Update changes the spinner message
func (s *Spinner) Update(message string) {
	s.message = message
}

// Complete stops the spinner with a success message
func (s *Spinner) Complete(message string) {
	s.Stop()
	fmt.Fprintf(s.writer, "\n✅ %s\n", message)
}

// Fail stops the spinner with a failure message
func (s *Spinner) Fail(message string) {
	s.Stop()
	fmt.Fprintf(s.writer, "\n❌ %s\n", message)
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	if s.active {
		s.active = false
		s.stopCh <- true
		fmt.Fprint(s.writer, "\n") // New line instead of clearing
	}
}

// Dots creates a dots progress indicator
type Dots struct {
	writer  io.Writer
	message string
	active  bool
	stopCh  chan bool
}

// NewDots creates a new dots progress indicator
func NewDots() *Dots {
	return &Dots{
		writer: os.Stdout,
		stopCh: make(chan bool, 1),
	}
}

// Start begins the dots indicator
func (d *Dots) Start(message string) {
	d.message = message
	d.active = true
	
	fmt.Fprint(d.writer, message)
	
	go func() {
		count := 0
		for {
			select {
			case <-d.stopCh:
				return
			default:
				if d.active {
					fmt.Fprint(d.writer, ".")
					count++
					if count%3 == 0 {
						// Reset dots
						fmt.Fprint(d.writer, "\r"+d.message)
					}
					time.Sleep(500 * time.Millisecond)
				}
			}
		}
	}()
}

// Update changes the dots message
func (d *Dots) Update(message string) {
	d.message = message
	if d.active {
		fmt.Fprintf(d.writer, "\n%s", message)
	}
}

// Complete stops the dots with a success message
func (d *Dots) Complete(message string) {
	d.Stop()
	fmt.Fprintf(d.writer, " ✅ %s\n", message)
}

// Fail stops the dots with a failure message
func (d *Dots) Fail(message string) {
	d.Stop()
	fmt.Fprintf(d.writer, " ❌ %s\n", message)
}

// Stop stops the dots indicator
func (d *Dots) Stop() {
	if d.active {
		d.active = false
		d.stopCh <- true
	}
}

// ProgressBar creates a visual progress bar
type ProgressBar struct {
	writer   io.Writer
	message  string
	total    int
	current  int
	width    int
	active   bool
	stopCh   chan bool
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int) *ProgressBar {
	return &ProgressBar{
		writer: os.Stdout,
		total:  total,
		width:  40,
		stopCh: make(chan bool, 1),
	}
}

// Start begins the progress bar
func (p *ProgressBar) Start(message string) {
	p.message = message
	p.active = true
	p.current = 0
	p.render()
}

// Update advances the progress bar
func (p *ProgressBar) Update(message string) {
	if p.current < p.total {
		p.current++
	}
	p.message = message
	p.render()
}

// SetProgress sets specific progress value
func (p *ProgressBar) SetProgress(current int, message string) {
	p.current = current
	p.message = message
	p.render()
}

// Complete finishes the progress bar
func (p *ProgressBar) Complete(message string) {
	p.current = p.total
	p.message = message
	p.render()
	fmt.Fprintf(p.writer, " ✅ %s\n", message)
	p.Stop()
}

// Fail stops the progress bar with failure
func (p *ProgressBar) Fail(message string) {
	p.render()
	fmt.Fprintf(p.writer, " ❌ %s\n", message)
	p.Stop()
}

// Stop stops the progress bar
func (p *ProgressBar) Stop() {
	p.active = false
}

// render draws the progress bar
func (p *ProgressBar) render() {
	if !p.active {
		return
	}
	
	percent := float64(p.current) / float64(p.total)
	filled := int(percent * float64(p.width))
	
	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)
	
	fmt.Fprintf(p.writer, "\n%s [%s] %d%%", p.message, bar, int(percent*100))
}

// Static creates a simple static progress indicator
type Static struct {
	writer io.Writer
}

// NewStatic creates a new static progress indicator
func NewStatic() *Static {
	return &Static{
		writer: os.Stdout,
	}
}

// Start shows the initial message
func (s *Static) Start(message string) {
	fmt.Fprintf(s.writer, "→ %s", message)
}

// Update shows an update message
func (s *Static) Update(message string) {
	fmt.Fprintf(s.writer, " - %s", message)
}

// Complete shows completion message
func (s *Static) Complete(message string) {
	fmt.Fprintf(s.writer, " ✅ %s\n", message)
}

// Fail shows failure message
func (s *Static) Fail(message string) {
	fmt.Fprintf(s.writer, " ❌ %s\n", message)
}

// Stop does nothing for static indicator
func (s *Static) Stop() {
	// No-op for static indicator
}

// LineByLine creates a line-by-line progress indicator
type LineByLine struct {
	writer io.Writer
	silent bool
}

// NewLineByLine creates a new line-by-line progress indicator
func NewLineByLine() *LineByLine {
	return &LineByLine{
		writer: os.Stdout,
		silent: false,
	}
}

// Light creates a minimal progress indicator with just essential status
type Light struct {
	writer io.Writer
	silent bool
}

// NewLight creates a new light progress indicator
func NewLight() *Light {
	return &Light{
		writer: os.Stdout,
		silent: false,
	}
}

// NewQuietLineByLine creates a quiet line-by-line progress indicator
func NewQuietLineByLine() *LineByLine {
	return &LineByLine{
		writer: os.Stdout,
		silent: true,
	}
}

// Start shows the initial message
func (l *LineByLine) Start(message string) {
	fmt.Fprintf(l.writer, "\n🔄 %s\n", message)
}

// Update shows an update message
func (l *LineByLine) Update(message string) {
	if !l.silent {
		fmt.Fprintf(l.writer, "   %s\n", message)
	}
}

// Complete shows completion message
func (l *LineByLine) Complete(message string) {
	fmt.Fprintf(l.writer, "✅ %s\n\n", message)
}

// Fail shows failure message
func (l *LineByLine) Fail(message string) {
	fmt.Fprintf(l.writer, "❌ %s\n\n", message)
}

// Stop does nothing for line-by-line (no cleanup needed)
func (l *LineByLine) Stop() {
	// No cleanup needed for line-by-line
}

// Light indicator methods - minimal output
func (l *Light) Start(message string) {
	if !l.silent {
		fmt.Fprintf(l.writer, "▶ %s\n", message)
	}
}

func (l *Light) Update(message string) {
	if !l.silent {
		fmt.Fprintf(l.writer, "  %s\n", message)
	}
}

func (l *Light) Complete(message string) {
	if !l.silent {
		fmt.Fprintf(l.writer, "✓ %s\n", message)
	}
}

func (l *Light) Fail(message string) {
	if !l.silent {
		fmt.Fprintf(l.writer, "✗ %s\n", message)
	}
}

func (l *Light) Stop() {
	// No cleanup needed for light indicator
}

// NewIndicator creates an appropriate progress indicator based on environment
func NewIndicator(interactive bool, indicatorType string) Indicator {
	if !interactive {
		return NewLineByLine() // Use line-by-line for non-interactive mode
	}
	
	switch indicatorType {
	case "spinner":
		return NewSpinner()
	case "dots":
		return NewDots()
	case "bar":
		return NewProgressBar(100) // Default to 100 steps
	case "line":
		return NewLineByLine()
	case "light":
		return NewLight()
	default:
		return NewLineByLine() // Default to line-by-line for better compatibility
	}
}

// NullIndicator is a no-op indicator that produces no output (for TUI mode)
type NullIndicator struct{}

// NewNullIndicator creates an indicator that does nothing
func NewNullIndicator() *NullIndicator {
	return &NullIndicator{}
}

func (n *NullIndicator) Start(message string)    {}
func (n *NullIndicator) Update(message string)   {}
func (n *NullIndicator) Complete(message string) {}
func (n *NullIndicator) Fail(message string)     {}
func (n *NullIndicator) Stop()                   {}