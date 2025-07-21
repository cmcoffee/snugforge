package nfo

import (
	"fmt"
	. "github.com/cmcoffee/snugforge/xsync"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Holds transfer display related data, including a list of monitors, update lock, and display flag.
var transferDisplay struct {
	update_lock sync.RWMutex
	display     int64
	monitors    []*tmon
}

// ReadSeekCloser is an interface that wraps the basic Read, Seek, and
// Close methods. It allows reading from and seeking within a data source,
// and then closing the source when finished.
type ReadSeekCloser interface {
	Seek(offset int64, whence int) (int64, error)
	Read(p []byte) (n int, err error)
	Close() error
}

// nopSeeker implements ReadSeekCloser by ignoring all Seek calls.
type nopSeeker struct {
	io.ReadCloser
}

// Seek always returns 0, nil.
func (T nopSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

// NopSeeker returns a ReadSeekCloser that does not perform seeking.
// It wraps the provided io.ReadCloser and always returns 0, nil from Seek.
func NopSeeker(input io.ReadCloser) ReadSeekCloser {
	return &nopSeeker{input}
}

// termWidth returns the width of the terminal.
// It returns 0 if the terminal size cannot be determined.
func termWidth() int {
	width, _, _ := terminal.GetSize(int(syscall.Stderr))
	width--
	if width < 1 {
		width = 0
	}
	return width
}

// LeftToRight displays the progress bar from left to right.
// RightToLeft displays the progress bar from right to left.
// NoRate prevents the display of the transfer rate.
// MaxWidth scales the progress bar width to maximum.
// ProgressBarSummary maintains the progress bar after completion.
// NoSummary suppresses the summary log after completion.
// internal is for internal use only.
// trans_active indicates an active transfer.
// trans_closed indicates a closed transfer.
// trans_complete indicates a completed transfer.
// trans_error indicates a transfer error.
const (
	LeftToRight        = 1 << iota // Display progress bar left to right. (Default Behavior)
	RightToLeft                    // Display progress bar right to left.
	NoRate                         // Do not show transfer rate, left to right.
	MaxWidth                       // Scale width to maximum.
	ProgressBarSummary             // Maintain progress bar when transfer complete.
	NoSummary                      // Do not log a summary after completion.
	internal
	trans_active
	trans_closed
	trans_complete
	trans_error
)

// readSeekCounter wraps a ReadSeekCloser and counts bytes read.
type readSeekCounter struct {
	counter func(int)
	ReadSeekCloser
}

// Read reads from the underlying ReadSeekCloser and updates the counter.
func (r readSeekCounter) Read(p []byte) (n int, err error) {
	n, err = r.ReadSeekCloser.Read(p)
	r.counter(n)
	return
}

// TransferCounter wraps a ReadSeekCloser and counts the number of bytes read.
// It returns a new ReadSeekCloser that calls the provided counter function
// after each Read operation with the number of bytes read.
func TransferCounter(input ReadSeekCloser, counter func(int)) ReadSeekCloser {
	return readSeekCounter{
		counter,
		input,
	}
}

// TransferMonitor creates a transfer monitor and starts displaying
// transfer progress. It accepts the name of the transfer, the total
// size, flags, the source, and an optional prefix. It returns a
// ReadSeekCloser that wraps the original source and allows monitoring
// of the transfer.
func TransferMonitor(name string, total_size int64, flag int, source ReadSeekCloser, optional_prefix ...string) ReadSeekCloser {
	transferDisplay.update_lock.Lock()
	defer transferDisplay.update_lock.Unlock()

	var (
		short_name  []rune
		target_size int
		prefix      string
	)

	if len(optional_prefix) > 0 {
		prefix = optional_prefix[0]
	}

	b_flag := BitFlag(flag)
	if b_flag.Has(LeftToRight) || b_flag <= 0 {
		b_flag.Set(LeftToRight)
	}

	if b_flag.Has(internal) {
		b_flag.Set(NoRate | NoSummary)
	}

	if !b_flag.Has(NoRate) {
		target_size = 25
	} else {
		target_size = 40
	}

	for i, v := range name {
		if i < target_size {
			short_name = append(short_name, v)
		} else {
			short_name = append(short_name, []rune("..")[0:]...)
			break
		}
	}

	if len(short_name) < target_size && (!b_flag.Has(internal) && !b_flag.Has(ProgressBarSummary)) {
		x := len(short_name) - 1
		var y []rune
		for i := 0; i <= target_size-x; i++ {
			y = append(y, ' ')
		}
		short_name = append(y[0:], short_name[0:]...)
	}

	b_flag.Set(trans_active)

	tm := &tmon{
		flag:        b_flag,
		name:        name,
		prefix:      prefix,
		short_name:  string(short_name),
		total_size:  total_size,
		transferred: 0,
		offset:      0,
		rate:        "0.0bps",
		start_time:  time.Now(),
		source:      source,
	}

	var spin_index int
	spin_txt := []string{"\\", "|", "/", "-"}

	spinner := func() string {
		if spin_index < len(spin_txt)-1 {
			spin_index++
		} else {
			spin_index = 0
		}
		return fmt.Sprintf(spin_txt[spin_index])
	}

	transferDisplay.monitors = append(transferDisplay.monitors, tm)

	if len(transferDisplay.monitors) == 1 {
		PleaseWait.flag.Set(transfer_monitor_active)
		transferDisplay.display = 1

		go func() {
			for {
				transferDisplay.update_lock.Lock()

				var monitors []*tmon

				// Clean up transfers.
				for i := len(transferDisplay.monitors) - 1; i >= 0; i-- {
					if transferDisplay.monitors[i].flag.Has(trans_closed) {
						transferDisplay.monitors = append(transferDisplay.monitors[:i], transferDisplay.monitors[i+1:]...)
					} else {
						monitors = append(monitors, transferDisplay.monitors[i])
					}
				}

				if len(transferDisplay.monitors) == 0 {
					PleaseWait.flag.Unset(transfer_monitor_active)
					transferDisplay.update_lock.Unlock()
					return
				}

				transferDisplay.update_lock.Unlock()

				// Display transfers.
				for _, v := range monitors {
					for i := 0; i < 10; i++ {
						if v.flag.Has(trans_active) {
							Flash("[%s] %s", spinner(), v.showTransfer(false))
						} else {
							break
						}
						time.Sleep(time.Millisecond * 200)
					}
				}
			}
		}()

	}

	return tm
}

// Seek moves the current position in the underlying source.
// It returns the new offset and any error that occurred.
func (tm *tmon) Seek(offset int64, whence int) (int64, error) {
	o, err := tm.source.Seek(offset, whence)
	tm.transferred = o
	tm.offset = o
	return o, err
}

// Read reads from the underlying source. It updates the transferred
// count and handles potential errors, closing the transfer if necessary.
func (tm *tmon) Read(p []byte) (n int, err error) {
	n, err = tm.source.Read(p)
	atomic.StoreInt64(&tm.transferred, atomic.LoadInt64(&tm.transferred)+int64(n))
	if err != nil {
		if tm.flag.Has(trans_closed) {
			return
		}
		tm.flag.Set(trans_closed | trans_error)
		if tm.transferred == 0 {
			return
		}
	}
	return
}

// Close closes the underlying source and logs a transfer summary if needed.
func (tm *tmon) Close() error {
	tm.flag.Set(trans_closed)
	if (tm.transferred > 0 || tm.total_size == 0) && !tm.flag.Has(NoSummary) {
		Log(tm.showTransfer(true))
	}
	return tm.source.Close()
}

// spacePrint prints the input string padded with spaces.
// It pads the string with spaces to the minimum length.
func spacePrint(min int, input string) string {
	output := make([]rune, min)
	for i := 0; i < len(output); i++ {
		output[i] = ' '
	}
	return string(append(output[len(input)-1:], []rune(input)[0:]...))
}

// tmon represents a transfer monitor.
// It tracks the progress of a transfer and provides information
// such as transferred bytes, total size, and transfer rate.
type tmon struct {
	flag        BitFlag
	prefix      string
	name        string
	short_name  string
	total_size  int64
	transferred int64
	offset      int64
	rate        string
	chunk_size  int64
	start_time  time.Time
	source      ReadSeekCloser
}

// showTransfer returns a string representation of the transfer progress.
// It displays either a progress bar or transfer rate and size.
// The 'summary' parameter controls whether the full name or short name
// is used in the output.
func (t *tmon) showTransfer(summary bool) string {
	transferred := atomic.LoadInt64(&t.transferred)
	rate := t.showRate()

	var name string

	if summary {
		t.flag.Unset(trans_active)
		name = t.name
	} else {
		name = t.short_name
	}

	// 35 + 8 +8 + 8 + 8
	if t.total_size > -1 {
		return fmt.Sprintf("%s", t.progressBar(name))
	} else {
		return fmt.Sprintf("%s: %s (%s) ", t.name, rate, HumanSize(transferred))
	}
}

// showRate returns the current transfer rate as a string.
// It calculates the rate based on transferred bytes and elapsed time.
// Returns the formatted rate string (e.g., "1.2mbps") or "0.0bps" if no
// data has been transferred or transfer is not active. It also updates
// the internal rate and flags transfer completion if appropriate.
func (t *tmon) showRate() (rate string) {

	transferred := atomic.LoadInt64(&t.transferred)
	if transferred == 0 || t.flag.Has(trans_complete) {
		return t.rate
	}

	since := time.Since(t.start_time).Seconds()
	if since < 0.1 {
		since = 0.1
	}

	sz := float64(transferred-t.offset) * 8 / since

	names := []string{
		"bps",
		"kbps",
		"mbps",
		"gbps",
	}

	suffix := 0

	for sz >= 1000 && suffix < len(names)-1 {
		sz = sz / 1000
		suffix++
	}

	if sz != 0.0 {
		rate = fmt.Sprintf("%.1f%s", sz, names[suffix])
	} else {
		if t.flag.Has(trans_active) {
			rate = "0.0bps"
		} else {
			rate = "\b"
		}
	}

	t.rate = rate

	if !t.flag.Has(trans_complete) && atomic.LoadInt64(&t.transferred)+t.offset == t.total_size {
		t.flag.Set(trans_complete)
	}

	return t.rate
}

// progressBar formats and returns a progress bar string.
func (t *tmon) progressBar(name string) string {
	num := int((float64(atomic.LoadInt64(&t.transferred)) / float64(t.total_size)) * 100)

	if t.total_size == 0 {
		num = 100
	}

	sz := termWidth() - 3
	if !t.flag.Has(MaxWidth) && sz > 100 {
		sz = 100
	}

	var first_half, second_half string

	if !t.flag.Has(NoRate) {
		first_half = fmt.Sprintf("%s: %s", name, t.showRate())
		second_half = fmt.Sprintf("(%s/%s)", HumanSize(t.transferred), HumanSize(t.total_size))
	} else {
		first_half = fmt.Sprintf("%s:", name)
	}

	sz = sz - len(first_half) - len(second_half) - 15

	if t.flag.Has(trans_closed) && !t.flag.Has(ProgressBarSummary) && !t.flag.Has(NoSummary) || sz <= 0 {
		sz = 10
	}

	create_display := func(num, sz int) []rune {
		var left, right, done, blank rune

		if !t.flag.Has(NoRate) {
			right = '>'
			left = '<'
			done = '='
			blank = ' '
		} else {
			right = '#'
			left = '#'
			done = '#'
			blank = '.'
		}

		display := make([]rune, sz)
		x := num * sz / 100

		if !t.flag.Has(RightToLeft) {
			for n := range display {
				if n < x {
					if n+1 < x {
						display[n] = done
					} else {
						display[n] = right
					}
				} else {
					display[n] = blank
				}
			}
		} else {
			x = sz - x - 1
			for n := range display {
				if n > x {
					if n-1 > x {
						display[n] = done
					} else {
						display[n] = left
					}
				} else {
					display[n] = blank
				}
			}
		}
		return display
	}

	display := create_display(num, sz)

	if sz > 10 {
		return fmt.Sprintf("%s [%s] %d%% %s ", first_half, string(display[0:]), int(num), second_half)
	} else {
		if t.flag.Has(trans_closed) {
			return fmt.Sprintf("%s%s %d%% %s", t.prefix, first_half, int(num), second_half)
		} else {
			return fmt.Sprintf("%s %d%% %s", first_half, int(num), second_half)
		}
	}
}
