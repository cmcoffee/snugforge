package wrotate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// rotaFile represents a rotating file writer.
type rotaFile struct {
	name        string
	flag        uint32
	file        *os.File
	buffer      bytes.Buffer
	rError      error
	maxBytes    int64
	bytesLeft   int64
	maxRotation uint
	writeLock   sync.Mutex
	rotatorWg   sync.WaitGroup
}

// toBuffer represents the buffer destination.
// toFile represents the file destination.
// stateFailed represents a failed operation.
// stateClosed indicates that the file is closed.
const (
	toBuffer = iota
	toFile
	stateFailed
	stateClosed
)

// Write writes the provided byte slice to the underlying storage.
// It handles file rotation and switching between file and buffer.
func (f *rotaFile) Write(p []byte) (n int, err error) {
	f.writeLock.Lock()
	defer f.writeLock.Unlock()

	switch atomic.LoadUint32(&f.flag) {
	case toFile:
		if f.bytesLeft < 0 {
			// Rotate files in background while writing to memory.
			atomic.StoreUint32(&f.flag, toBuffer)
			f.rotatorWg.Add(1)
			go f.rotator()
			return f.buffer.Write(p)
		}
		n, err = f.file.Write(p)
		f.bytesLeft -= int64(n)
		return
	case toBuffer:
		return f.buffer.Write(p)
	case stateClosed:
		return 0, os.ErrClosed
	case stateFailed:
		return 0, f.rError
	}
	return
}

// OpenFile opens or creates a file, optionally rotating it based on size and rotations.
// It returns a WriteCloser and an error if file opening fails.
func OpenFile(name string, maxBytes int64, maxRotations uint) (io.WriteCloser, error) {
	// Return a plain file when rotation is disabled.
	if maxBytes <= 0 || maxRotations <= 0 {
		return os.OpenFile(name, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	}

	rotator := &rotaFile{
		name:        name,
		flag:        toFile,
		maxBytes:    maxBytes,
		maxRotation: maxRotations,
	}

	var err error
	rotator.file, err = os.OpenFile(name, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		rotator.rotatorWg.Add(1)
		rotator.rotator() // Attempt to rotate file if we cannot open it.
		if rotator.rError != nil {
			return nil, rotator.rError
		}
		return rotator, nil
	}

	finfo, err := rotator.file.Stat()
	if err != nil {
		return nil, err
	}

	rotator.bytesLeft = rotator.maxBytes - finfo.Size()

	return rotator, nil
}

// Close waits for any in-progress rotation to complete, then closes the underlying file.
func (R *rotaFile) Close() error {
	R.rotatorWg.Wait()
	atomic.StoreUint32(&R.flag, stateClosed)
	return R.file.Close()
}

func (R *rotaFile) rotator() {
	defer R.rotatorWg.Done()

	fpath, fname := filepath.Split(R.name)
	if fpath == "" {
		fpath = "." + string(os.PathSeparator)
	}

	// chkErr sets the error state and returns true if err is non-nil.
	chkErr := func(err error) bool {
		if err != nil {
			R.rError = err
			atomic.StoreUint32(&R.flag, stateFailed)
			return true
		}
		return false
	}

	if R.file != nil {
		if chkErr(R.file.Close()) {
			return
		}
	}

	flist, err := os.ReadDir(fpath)
	if chkErr(err) {
		return
	}

	// Collect rotation candidates: exact name or name.N (digits-only suffix).
	files := make(map[string]struct{})
	for _, v := range flist {
		n := v.Name()
		if n == fname || (strings.HasPrefix(n, fname+".") && isNumericSuffix(n[len(fname)+1:])) {
			files[n] = struct{}{}
		}
	}

	fileCount := uint(len(files))

	// Rename existing rotations from highest to lowest, dropping any beyond maxRotation.
	for i := fileCount; i > 0; i-- {
		target := fname
		if i > 1 {
			target = fmt.Sprintf("%s.%d", fname, i-1)
		}
		if _, ok := files[target]; !ok {
			continue
		}
		if i > R.maxRotation {
			if chkErr(os.Remove(filepath.Join(fpath, target))) {
				return
			}
		} else {
			src := filepath.Join(fpath, target)
			dst := filepath.Join(fpath, fmt.Sprintf("%s.%d", fname, i))
			if chkErr(os.Rename(src, dst)) {
				return
			}
		}
	}

	// Open new file.
	R.file, err = os.OpenFile(R.name, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if chkErr(err) {
		return
	}

	// Drain the in-memory buffer to the new file. Hold writeLock only briefly
	// for each copy to avoid stalling writers during disk I/O. The flag stays
	// toBuffer until the drain is complete, so concurrent writes accumulate in
	// the buffer and are picked up in subsequent iterations.
	var totalWritten int64
	for {
		R.writeLock.Lock()
		if R.buffer.Len() == 0 {
			R.bytesLeft = R.maxBytes - totalWritten
			atomic.StoreUint32(&R.flag, toFile)
			R.writeLock.Unlock()
			break
		}
		data := make([]byte, R.buffer.Len())
		copy(data, R.buffer.Bytes())
		R.buffer.Reset()
		R.writeLock.Unlock()

		n, werr := R.file.Write(data)
		totalWritten += int64(n)
		if chkErr(werr) {
			return
		}
	}
}

// isNumericSuffix reports whether s is a non-empty string of ASCII digits.
func isNumericSuffix(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
