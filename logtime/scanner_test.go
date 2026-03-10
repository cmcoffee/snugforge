package logtime

import (
	"strings"
	"testing"
	"time"
)

const testLog = `2026-03-05 01:00:00.000 Server starting up
2026-03-05 01:00:01.000 Loading configuration
  config file: /etc/app.conf
  debug mode: true
2026-03-05 01:00:02.000 Listening on port 8080
2026-03-05 02:30:00.000 Request received from 10.0.0.1
  GET /api/status
  Headers:
    Accept: application/json
2026-03-05 02:30:01.000 Response sent: 200 OK
2026-03-05 04:00:00.000 Shutting down
`

func TestScannerFullRange(t *testing.T) {
	start := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	stop := time.Date(2026, 3, 5, 23, 59, 59, 0, time.UTC)

	sc, err := NewScanner(start, stop, strings.NewReader(testLog))
	if err != nil {
		t.Fatal(err)
	}

	var entries []string
	for sc.Scan() {
		entries = append(entries, sc.Text())
	}
	if sc.Err() != nil {
		t.Fatal(sc.Err())
	}

	if len(entries) != 6 {
		t.Fatalf("expected 6 entries, got %d", len(entries))
	}

	// Second entry should contain continuation lines.
	if !strings.Contains(entries[1], "debug mode: true") {
		t.Errorf("entry 1 missing continuation lines: %q", entries[1])
	}

	// Fourth entry should have multi-line continuation.
	if !strings.Contains(entries[3], "Accept: application/json") {
		t.Errorf("entry 3 missing continuation lines: %q", entries[3])
	}
}

func TestScannerTimeWindow(t *testing.T) {
	start := time.Date(2026, 3, 5, 2, 0, 0, 0, time.UTC)
	stop := time.Date(2026, 3, 5, 3, 0, 0, 0, time.UTC)

	sc, err := NewScanner(start, stop, strings.NewReader(testLog))
	if err != nil {
		t.Fatal(err)
	}

	var entries []string
	var times []time.Time
	for sc.Scan() {
		entries = append(entries, sc.Text())
		times = append(times, sc.Time())
	}
	if sc.Err() != nil {
		t.Fatal(sc.Err())
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries in window, got %d", len(entries))
	}

	want := time.Date(2026, 3, 5, 2, 30, 0, 0, time.UTC)
	if !times[0].Equal(want) {
		t.Errorf("first entry time = %v, want %v", times[0], want)
	}

	if !strings.Contains(entries[0], "Accept: application/json") {
		t.Errorf("first entry should include continuation lines: %q", entries[0])
	}
}

func TestScannerStopsEarly(t *testing.T) {
	// Stop time before the last entry; scanner should not read further.
	start := time.Date(2026, 3, 5, 1, 0, 0, 0, time.UTC)
	stop := time.Date(2026, 3, 5, 1, 0, 1, 500000000, time.UTC)

	sc, err := NewScanner(start, stop, strings.NewReader(testLog))
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for sc.Scan() {
		count++
	}
	if sc.Err() != nil {
		t.Fatal(sc.Err())
	}

	if count != 2 {
		t.Fatalf("expected 2 entries before stop, got %d", count)
	}
}

func TestScannerBracketFormat(t *testing.T) {
	log := `[05-Mar-2026 10:00:00 UTC] Starting process
[05-Mar-2026 10:00:01 UTC] Step 1 complete
  details: all good
[05-Mar-2026 10:00:02 UTC] Done
`
	start := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	stop := time.Date(2026, 3, 5, 10, 0, 1, 999000000, time.UTC)

	sc, err := NewScanner(start, stop, strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	var entries []string
	for sc.Scan() {
		entries = append(entries, sc.Text())
	}
	if sc.Err() != nil {
		t.Fatal(sc.Err())
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if !strings.Contains(entries[1], "details: all good") {
		t.Errorf("entry 1 missing continuation: %q", entries[1])
	}
}

func TestScannerNoMatch(t *testing.T) {
	start := time.Date(2026, 3, 5, 23, 0, 0, 0, time.UTC)
	stop := time.Date(2026, 3, 5, 23, 59, 0, 0, time.UTC)

	sc, err := NewScanner(start, stop, strings.NewReader(testLog))
	if err != nil {
		t.Fatal(err)
	}

	if sc.Scan() {
		t.Error("expected no entries in empty time window")
	}
}

func TestScannerNoTimestamps(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stop := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	_, err := NewScanner(start, stop, strings.NewReader("not-a-timestamp\nstill not\n"))
	if err == nil {
		t.Error("expected error when no timestamped lines found")
	}
}

func TestScannerEmptyInput(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stop := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	_, err := NewScanner(start, stop, strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestScannerLeadingNonTimestampLines(t *testing.T) {
	log := `=== Log Start ===
Some header info
2026-03-05 01:00:00.000 First real entry
2026-03-05 01:00:01.000 Second entry
`
	start := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	stop := time.Date(2026, 3, 5, 23, 59, 0, 0, time.UTC)

	sc, err := NewScanner(start, stop, strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for sc.Scan() {
		count++
	}

	if count != 2 {
		t.Fatalf("expected 2 entries (skipping header), got %d", count)
	}
}
