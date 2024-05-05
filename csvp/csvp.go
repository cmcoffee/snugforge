package csvp

import (
	"bufio"
	"encoding/csv"
	"github.com/cmcoffee/snugforge/swapreader"
	"io"
	"strings"
)

type rowReadError error
type rowProcessError error

type CSVReader struct {
	Processor    func(row []string) (err error)                     // Callback funcction for each row read.
	ErrorHandler func(line int, row string, err error) (abort bool) // ErrorHandler when problem reading CSV or processing CSV.
}

// Allocates a New CSVReader.
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

// Returns true if error is generatored from reading the CSV.
func IsReadError(err error) bool {
	if _, ok := err.(rowReadError); ok {
		return true
	}
	return false
}

// Returns true if error is generated from processing the row of the CSV.
func IsRowError(err error) bool {
	if _, ok := err.(rowProcessError); ok {
		return true
	}
	return false
}

// Reads incoming CSV data.
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
