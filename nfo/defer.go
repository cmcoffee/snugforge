package nfo

import (
	"crypto/rand"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"
)

var (
	// SignalChan is a channel used to receive OS signals for graceful shutdown.
	// It allows the application to respond to signals like SIGINT, SIGTERM, and SIGKILL.
	signalChan = make(chan os.Signal)

	// globalDefer holds the deferred functions to be executed during shutdown.
	// It uses a mutex to protect concurrent access to the deferred functions list.
	globalDefer struct {
		mutex sync.RWMutex
		ids   []string
		d_map map[string]func() error
	}

	// errCode stores the exit code for the application.
	// It is updated based on the signal received or other error conditions.
	errCode = 0

	// wait is a WaitGroup used to wait for all deferred functions to complete before exiting.
	wait sync.WaitGroup

	// exit_lock is a channel used to signal the shutdown goroutine to exit after all deferred functions have completed.
	exit_lock = make(chan struct{})
)

// ShutdownInProgress reports whether a shutdown is in progress.
// It checks the value of the fatal_triggered atomic integer.
func ShutdownInProgress() bool {
	if atomic.LoadInt32(&fatal_triggered) != 0 {
		return true
	}
	return false
}

// BlockShutdown increments the WaitGroup counter, blocking shutdown
// until Counter() becomes zero.
func BlockShutdown() {
	wait.Add(1)
}

// UnblockShutdown signals the completion of a shutdown process.
// It decrements a WaitGroup counter, potentially unblocking
// a waiting shutdown routine.
func UnblockShutdown() {
	wait.Done()
}

// Defer registers a function to be called when all deferred functions have returned.
// It accepts a closer interface which can be either a func() or func() error.
// The function returns a cleanup function that, when called, removes the registered function from the defer list and executes it.
func Defer(closer interface{}) func() error {
	globalDefer.mutex.Lock()
	defer globalDefer.mutex.Unlock()

	errorWrapper := func(closerFunc func()) func() error {
		return func() error {
			closerFunc()
			return nil
		}
	}

	var id string

	for {
		// Generates a random tag.
		id = func(ch string) string {
			chlen := len(ch)

			rand_string := make([]byte, 32)
			rand.Read(rand_string)

			for i, v := range rand_string {
				rand_string[i] = ch[v%byte(chlen)]
			}
			return string(rand_string)
		}("!@#$%^&*()_+-=][{}|/.,><abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

		// Check if tag is used already
		if _, ok := globalDefer.d_map[id]; !ok {
			break
		}
	}

	var d func() error

	switch closer := closer.(type) {
	case func():
		d = errorWrapper(closer)
	case func() error:
		d = closer
	default:
		return nil
	}

	globalDefer.ids = append(globalDefer.ids, id)
	globalDefer.d_map[id] = d

	return func() error {
		globalDefer.mutex.Lock()
		defer globalDefer.mutex.Unlock()
		delete(globalDefer.d_map, id)
		for i := len(globalDefer.ids) - 1; i > -1; i-- {
			if globalDefer.ids[i] == id {
				globalDefer.ids = append(globalDefer.ids[:i], globalDefer.ids[i+1:]...)
			}
		}
		return d()
	}
}

// Exit terminates the program with the given exit code.
func Exit(exit_code int) {
	if r := recover(); r != nil {
		write2log(FATAL|_bypass_lock, "(panic) %s", string(debug.Stack()))
		os.Exit(1)
	}
	atomic.StoreInt32(&fatal_triggered, 2) // Ignore any Fatal() calls, we've been told to exit.
	signalChan <- os.Kill
	<-exit_lock
	os.Exit(exit_code)
}

// SetSignals sets the signals to be notified on.
// It stops any existing signal notifications and registers the provided signals.
func SetSignals(sig ...os.Signal) {
	mutex.Lock()
	defer mutex.Unlock()
	signal.Stop(signalChan)
	signal.Notify(signalChan, sig...)
}

// SignalCallback registers a callback function to be executed when a
// specific OS signal is received.
func SignalCallback(signal os.Signal, callback func() (continue_shutdown bool)) {
	mutex.Lock()
	defer mutex.Unlock()
	callbacks[signal] = callback
}

// callbacks is a map of OS signals to functions that handle them.
var callbacks = make(map[os.Signal]func() bool)

// init initializes global resources and sets up signal handling for graceful shutdown.
func init() {
	globalDefer.d_map = make(map[string]func() error)
	SetSignals(syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for {
			s := <-signalChan

			mutex.Lock()
			cb := callbacks[s]
			mutex.Unlock()

			if cb != nil {
				if !cb() {
					continue
				}
			}

			atomic.CompareAndSwapInt32(&fatal_triggered, 0, 2)

			switch s {
			case syscall.SIGINT:
				errCode = 130
			case syscall.SIGHUP:
				errCode = 129
			case syscall.SIGTERM:
				errCode = 143
			}

			break
		}

		// Snapshot the IDs and functions under the lock so concurrent early-cleanup
		// calls cannot modify the map while we iterate.
		globalDefer.mutex.Lock()
		snapshot := make([]func() error, 0, len(globalDefer.ids))
		for i := len(globalDefer.ids) - 1; i >= 0; i-- {
			if fn, ok := globalDefer.d_map[globalDefer.ids[i]]; ok {
				snapshot = append(snapshot, fn)
			}
		}
		globalDefer.mutex.Unlock()

		// Run through all globalDefer functions outside the lock.
		for _, fn := range snapshot {
			if err := fn(); err != nil {
				write2log(ERROR|_bypass_lock, err.Error())
			}
		}

		// Wait on any process that have access to wait.
		wait.Wait()

		// Hide Please Wait
		PleaseWait.Hide()

		// Try to flush out any remaining text.
		write2log(_flash_txt|_no_logging|_bypass_lock, "")

		// Finally exit the application
		select {
		case exit_lock <- struct{}{}:
		default:
			os.Exit(errCode)
		}
	}()
}
