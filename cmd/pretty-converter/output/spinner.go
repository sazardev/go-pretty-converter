package output

import (
	"fmt"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type Spinner struct {
	message string
	done    chan struct{}
	ack     chan struct{}
	start   time.Time
}

func StartSpinner(message string) *Spinner {
	s := &Spinner{
		message: message,
		done:    make(chan struct{}),
		ack:     make(chan struct{}),
		start:   time.Now(),
	}
	go s.run()
	return s
}

func (s *Spinner) run() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	defer close(s.ack)

	i := 0
	for {
		select {
		case <-s.done:
			fmt.Print("\r\033[K")
			return
		case <-ticker.C:
			frame := StepRunningStyle.Render(spinnerFrames[i%len(spinnerFrames)])
			fmt.Printf("\r\033[K  %s %s", frame, s.message)
			i++
		}
	}
}

func (s *Spinner) Done(msg string) {
	close(s.done)
	<-s.ack
	fmt.Printf("  %s %s\n", SuccessSymbol, msg)
}

func (s *Spinner) Fail(msg string) {
	close(s.done)
	<-s.ack
	fmt.Printf("  %s %s\n", ErrorSymbol, ErrorStyle.Render(msg))
}

func (s *Spinner) Ok() {
	s.Done(s.message)
}

func (s *Spinner) Elapsed() time.Duration {
	return time.Since(s.start)
}
