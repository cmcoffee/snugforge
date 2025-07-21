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
	name         string
	flag         uint32
	file         *os.File
	buffer       bytes.Buffer
	r_error      error
	max_bytes    int64
	bytes_left   int64
	max_rotation uint
	write_lock   sync.Mutex
}

// to_BUFFER represents the buffer destination.
// to_FILE represents the file destination.
// _FAILED represents a failed operation.
// _CLOSED indicates that the file is closed.
const (
	to_BUFFER = iota
	to_FILE
	_FAILED
	_CLOSED
)

// Write writes the provided byte slice to the underlying storage.
// It handles file rotation and switching between file and buffer.
func (f *rotaFile) Write(p []byte) (n int, err error) {
	f.write_lock.Lock()
	defer f.write_lock.Unlock()

	switch atomic.LoadUint32(&f.flag) {
	case to_FILE:
		if f.bytes_left < 0 {
			// Rotate files in background while writing to memory.
			atomic.StoreUint32(&f.flag, to_BUFFER)
			go f.rotator()
			return f.buffer.Write(p)
		}
		n, err = f.file.Write(p)
		f.bytes_left = f.bytes_left - int64(n)
		return
	case to_BUFFER:
		return f.buffer.Write(p)
	case _CLOSED:
		return f.file.Write(p)
	case _FAILED:
		return -1, f.r_error
	}
	return
}

// OpenFile opens or creates a file, optionally rotating it based on size and rotations.
// It returns a WriteCloser and an error if file opening fails.
func OpenFile(name string, max_bytes int64, max_rotations uint) (io.WriteCloser, error) {
	rotator := &rotaFile{
		name:         name,
		flag:         to_FILE,
		r_error:      nil,
		max_bytes:    max_bytes,
		max_rotation: max_rotations,
	}

	var err error

	rotator.file, err = os.OpenFile(name, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		rotator.rotator() // Attempt to rotate file if we cannnot open it.
		if rotator.r_error != nil {
			return nil, rotator.r_error
		} else {
			return rotator, nil
		}
	}

	// Just return the open file if max_bytes <= 0 or max_rotations <= 0.
	if max_bytes <= 0 || max_rotations <= 0 {
		return rotator.file, nil
	}

	finfo, err := rotator.file.Stat()
	if err != nil {
		return nil, err
	}

	rotator.bytes_left = rotator.max_bytes - finfo.Size()

	return rotator, nil
}

// Close closes the underlying file and sets the flag to _CLOSED.
func (R *rotaFile) Close() (err error) {
	atomic.StoreUint32(&R.flag, _CLOSED)
	return R.file.Close()
}

func (R *rotaFile) rotator() {
	fpath, fname := filepath.Split(R.name)
	if fpath == "" {
		fpath = fmt.Sprintf(".%s", string(os.PathSeparator))
	}

	// Check on error, returns true if error triggered, false if not.
	chkErr := func(err error) bool {
		if err != nil {
			R.r_error = err
			atomic.StoreUint32(&R.flag, _FAILED)
			return true
		}
		return false
	}

	if R.file != nil {
		err := R.file.Close()
		if chkErr(err) {
			return
		}
	}

	flist, err := os.ReadDir(fpath)
	if chkErr(err) {
		return
	}

	files := make(map[string]os.FileInfo)

	for _, v := range flist {
		finfo, err := v.Info()
		if err != nil {
			return
		}
		if strings.Contains(v.Name(), fname) {
			files[v.Name()] = finfo
		}
	}

	file_count := uint(len(files))

	// Rename files
	for i := file_count; i > 0; i-- {
		target := fname

		if i > 1 {
			target = fmt.Sprintf("%s.%d", target, i-1)
		}

		if _, ok := files[target]; ok {
			if i > R.max_rotation {
				err = os.Remove(fmt.Sprintf("%s%s", fpath, target))
				if chkErr(err) {
					return
				}
			} else {
				err = os.Rename(fmt.Sprintf("%s%s", fpath, target), fmt.Sprintf("%s%s.%d", fpath, fname, i))
				if chkErr(err) {
					return
				}
			}
		}
	}

	// Open new file.
	R.file, err = os.OpenFile(R.name, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if chkErr(err) {
		return
	}

	R.write_lock.Lock()
	defer R.write_lock.Unlock()

	// Set l_files new size to new buffer.
	R.bytes_left = R.max_bytes - int64(R.buffer.Len())

	// Copy buffer to new file.
	_, err = io.Copy(R.file, &R.buffer)
	if chkErr(err) {
		return
	}

	R.buffer.Reset()

	// Switch Write function back to writing to file.
	atomic.StoreUint32(&R.flag, to_FILE)
	return
}
