package nfo

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cmcoffee/snugforge/xsync"
)

// init configures the PleaseWait loading animation.
// It sets the message and animation frames for a loading indicator.
func init() {
	PleaseWait.Set(func() string { return "Please wait ..." }, []string{"[>  ]", "[>> ]", "[>>>]", "[ >>]", "[  >]", "[  <]", "[ <<]", "[<<<]", "[<< ]", "[<  ]"})
}

// PleaseWait is a global variable representing the loading indicator.
var PleaseWait = new(loading)

// loading manages a loading indicator with customizable messages and animations.
type loading struct {
	flag    xsync.BitFlag
	message func() string
	anim_1  []string
	anim_2  []string
	mutex   sync.Mutex
	counter int32
}

// loading_backup holds the state needed to restore a loading animation.
// It captures the message and animation slices for later use.
type loading_backup struct {
	message func() string
	anim_1  []string
	anim_2  []string
}

// loading_show represents the flag for showing loading state.
// transfer_monitor_active represents the flag for active transfer monitor.
const (
	loading_show = 1 << iota
	transfer_monitor_active
)

// Restore sets the loading state to the backed-up values.
func (B *loading_backup) Restore() {
	PleaseWait.Set(B.message, B.anim_1, B.anim_2)
}

// Backup returns a copy of the loading state.
func (L *loading) Backup() *loading_backup {
	L.mutex.Lock()
	defer L.mutex.Unlock()
	return &loading_backup{L.message, L.anim_1, L.anim_2}
}

// Set configures the loading animation with a message and optional loader frames.
// It takes a function that returns the message to display and variadic slices
// representing the animation frames for the primary and secondary loaders.
// If no loader is provided, the animation is not started.
func (L *loading) Set(message func() string, loader ...[]string) {
	L.mutex.Lock()
	defer L.mutex.Unlock()

	if len(loader) == 0 {
		return
	}

	var anim_1, anim_2 []string

	anim_1 = loader[0]
	if len(loader) > 1 {
		anim_2 = loader[1]
	}

	if anim_2 == nil || len(anim_2) < len(anim_1) {
		anim_2 = make([]string, len(anim_1))
	}

	L.message = message
	L.anim_1 = anim_1
	L.anim_2 = anim_2
	count := atomic.AddInt32(&L.counter, 1)

	go func(message func() string, anim_1 []string, anim_2 []string, count int32) {
		for count == atomic.LoadInt32(&L.counter) {
			for i, str := range anim_1 {
				if L.flag.Has(loading_show) && !L.flag.Has(transfer_monitor_active) && count == atomic.LoadInt32(&L.counter) {
					Flash("%s %s %s", str, message(), anim_2[i])
				}
				time.Sleep(125 * time.Millisecond)
			}
		}
	}(message, anim_1, anim_2, count)
}

// Show enables the loading indicator.
func (L *loading) Show() {
	L.flag.Set(loading_show)
}

// Hide stops the loading animation and clears the output.
func (L *loading) Hide() {
	L.flag.Unset(loading_show)
	time.Sleep(time.Millisecond)
	Flash("")
}

// ProgressBar interface for tracking progress.
// Defines methods to add to, set, and mark progress as complete.
type ProgressBar interface {
	Add(num int) // Add num to progress bar.
	Set(num int) // Set num of progress bar.
	Done()       // Mark progress bar as complete.
}

// progressBar is a type that implements the ProgressBar interface,
// providing a progress bar based on a ReadSeekCloser.
type progressBar struct {
	tm ReadSeekCloser
}

// b_closer wraps a *bytes.Reader and implements the ReadSeekCloser interface.
// It provides a Close() method that does nothing.
type b_closer struct {
	*bytes.Reader
}

// Close closes the b_closer. It always returns nil.
// //
func (b b_closer) Close() error {
	return nil
}

// NewProgressBar creates a new progress bar with the given name and maximum value.
func NewProgressBar(name string, max int) ProgressBar {
	x := new(progressBar)
	var dummy b_closer
	dummy.Reader = bytes.NewReader(make([]byte, max))

	x.tm = TransferMonitor(name, int64(max), internal, dummy)
	return x
}

// Add increments the progress bar by the given number of bytes.
func (p *progressBar) Add(num int) {
	p.tm.Read(make([]byte, num))
}

// Set repositions the progress bar to the given number.
func (p *progressBar) Set(num int) {
	p.tm.Seek(int64(num), 0)
}

// Done closes the underlying ReadSeekCloser.
func (p *progressBar) Done() {
	p.tm.Close()
}
