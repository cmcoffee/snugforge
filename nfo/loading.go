package nfo

import (
	//"fmt"
	"bytes"
	"github.com/cmcoffee/snugforge/xsync"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	PleaseWait.Set(func() string { return "Please wait ..." }, []string{"[>  ]", "[>> ]", "[>>>]", "[ >>]", "[  >]", "[  <]", "[ <<]", "[<<<]", "[<< ]", "[<  ]"})
}

// PleaseWait is a wait prompt to display between requests.
var PleaseWait = new(loading)

type loading struct {
	flag    xsync.BitFlag
	message func() string
	anim_1  []string
	anim_2  []string
	mutex   sync.Mutex
	counter int32
}

type loading_backup struct {
	message func() string
	anim_1  []string
	anim_2  []string
}

const (
	loading_show = 1 << iota
	transfer_monitor_active
)

func (B *loading_backup) Restore() {
	PleaseWait.Set(B.message, B.anim_1, B.anim_2)
}

func (L *loading) Backup() *loading_backup {
	L.mutex.Lock()
	defer L.mutex.Unlock()
	return &loading_backup{L.message, L.anim_1, L.anim_2}
}

// Specify a "Please wait" animated PleaseWait line.
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

// Displays loader. "[>>>] Working, Please wait."
func (L *loading) Show() {
	L.flag.Set(loading_show)
}

// Hides display loader.
func (L *loading) Hide() {
	L.flag.Unset(loading_show)
	time.Sleep(time.Millisecond)
	Flash("")
}

type progressBar struct {
	name string
	max  int
	tm   ReadSeekCloser
}

type b_closer struct {
	*bytes.Reader
}

func (b b_closer) Close() error {
	return nil
}

// Updates loading to be a progress bar.
func ProgressBar(name string, max int) *progressBar {
	x := new(progressBar)
	x.max = max
	x.name = name
	var dummy b_closer
	dummy.Reader = bytes.NewReader(make([]byte, max))

	x.tm = TransferMonitor(name, int64(max), internal, dummy)
	return x
}

// Adds to progress bar.
func (p *progressBar) Add(num int) {
	p.tm.Read(make([]byte, num))
}

// Specify number set on progress bar.
func (p *progressBar) Set(num int) {
	p.tm.Seek(int64(num), 0)
}

// Complete progress bar, return to loading.
func (p *progressBar) Done() {
	p.tm.Close()
}
