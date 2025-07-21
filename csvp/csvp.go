package csvp

import (
	"bufio"
	"encoding/csv"
	"github.com/cmcoffee/snugforge/swapreader"
	"io"
	"strings"
)

// rowReadError is an error type for issues during CSV row reading.
type rowReadError error

// rowProcessError represents an error encountered while processing a row.
type rowProcessError error

// CSVReader processes CSV data row by row.
type CSVReader struct {
	Processor    func(row []string) (err error)                     // Callback funcction for each row read.
	ErrorHandler func(line int, row string, err error) (abort bool) // ErrorHandler when problem reading CSV or processing CSV.
}

// NewReader creates and returns a new CSVReader instance.
func NewReader() *CSVReader {
	return &CSVReader{
		func(row []string) (err error) {
			return nil
		},
		func(line int, input string, err error) (abort bool) {
			return false
		},
	}
}

// IsReadError reports whether err is a row read error.
// It checks if the error is of type rowReadError.
func IsReadError(err error) bool {
	if _, ok := err.(rowReadError); ok {
		return true
	}
	return false
}

// IsRowError checks if an error is a rowProcessError.
// It returns true if the error is of type rowProcessError,
// otherwise it returns false.
func IsRowError(err error) bool {
	if _, ok := err.(rowProcessError); ok {
		return true
	}
	return false
}

// Read reads a CSV from the provided reader, processing each row.
// It skips lines starting with '#'.
func (T *CSVReader) Read(reader io.Reader) {
	line := 0
	scanner := bufio.NewScanner(reader)
	swap := new(swapreader.Reader)
	csv_reader := csv.NewReader(swap)
	for scanner.Scan() {
		line++
		data := scanner.Bytes()
		if strings.HasPrefix(string(data), "#") {
			continue
		}
		swap.SetBytes(data)
		row, err := csv_reader.Read()
		if err != nil {
			if T.ErrorHandler != nil {
				if T.ErrorHandler(line, string(data), rowReadError(err)) {
					return
				}
			}
		}
		if T.Processor != nil {
			if err = T.Processor(row); err != nil {
				if T.ErrorHandler(line, string(data), rowProcessError(err)) {
					return
				}
			}
		}
	}
}
