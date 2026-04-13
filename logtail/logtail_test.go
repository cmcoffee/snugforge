package logtail

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// write_log is a helper that appends a line to a file.
func write_log(t *testing.T, path, line string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

func TestBasicPatternMatch(t *testing.T) {
	dir := t.TempDir()
	log_path := filepath.Join(dir, "app.log")

	// Create the file with an initial line so the tailer can open it.
	write_log(t, log_path, "2026-03-22 10:00:00 INFO startup complete")

	tail := Open(log_path)
	tail.SetInterval(50 * time.Millisecond)

	var mu sync.Mutex
	var matches []Match

	tail.MustOn(`ERROR (.+)`, func(m Match) {
		mu.Lock()
		matches = append(matches, m)
		mu.Unlock()
	})

	go func() {
		// Let the tailer start and seek to end.
		time.Sleep(150 * time.Millisecond)

		write_log(t, log_path, "2026-03-22 10:00:01 ERROR database connection lost")
		write_log(t, log_path, "2026-03-22 10:00:02 INFO request handled")
		write_log(t, log_path, "2026-03-22 10:00:03 ERROR disk full")

		time.Sleep(300 * time.Millisecond)
		tail.Close()
	}()

	if err := tail.Run(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	if matches[0].Text != "ERROR database connection lost" {
		t.Errorf("unexpected text: %s", matches[0].Text)
	}
	if matches[0].Groups[0] != "database connection lost" {
		t.Errorf("unexpected group: %s", matches[0].Groups[0])
	}
	if matches[0].Time.IsZero() {
		t.Error("expected parsed timestamp")
	}

	if matches[1].Text != "ERROR disk full" {
		t.Errorf("unexpected text: %s", matches[1].Text)
	}
}

func TestFromStart(t *testing.T) {
	dir := t.TempDir()
	log_path := filepath.Join(dir, "app.log")

	write_log(t, log_path, "2026-03-22 10:00:00 WARN low memory")
	write_log(t, log_path, "2026-03-22 10:00:01 WARN high cpu")

	tail := Open(log_path)
	tail.SetInterval(50 * time.Millisecond)
	tail.FromStart()

	var mu sync.Mutex
	var matches []Match

	tail.MustOn(`WARN (.+)`, func(m Match) {
		mu.Lock()
		matches = append(matches, m)
		mu.Unlock()
	})

	go func() {
		time.Sleep(300 * time.Millisecond)
		tail.Close()
	}()

	if err := tail.Run(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Groups[0] != "low memory" {
		t.Errorf("unexpected group: %s", matches[0].Groups[0])
	}
	if matches[1].Groups[0] != "high cpu" {
		t.Errorf("unexpected group: %s", matches[1].Groups[0])
	}
}

func TestFileTruncation(t *testing.T) {
	dir := t.TempDir()
	log_path := filepath.Join(dir, "app.log")

	write_log(t, log_path, "2026-03-22 10:00:00 INFO starting")

	tail := Open(log_path)
	tail.SetInterval(50 * time.Millisecond)

	var mu sync.Mutex
	var matches []Match

	tail.MustOn(`ERROR (.+)`, func(m Match) {
		mu.Lock()
		matches = append(matches, m)
		mu.Unlock()
	})

	go func() {
		time.Sleep(150 * time.Millisecond)

		write_log(t, log_path, "2026-03-22 10:00:01 ERROR first error")
		time.Sleep(300 * time.Millisecond)

		// Truncate the file (simulates copytruncate rotation).
		os.WriteFile(log_path, nil, 0644)
		time.Sleep(200 * time.Millisecond)

		// New content after truncation with a later timestamp.
		write_log(t, log_path, "2026-03-22 10:00:05 ERROR second error")
		time.Sleep(300 * time.Millisecond)

		tail.Close()
	}()

	if err := tail.Run(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Groups[0] != "first error" {
		t.Errorf("unexpected first match: %s", matches[0].Groups[0])
	}
	if matches[1].Groups[0] != "second error" {
		t.Errorf("unexpected second match: %s", matches[1].Groups[0])
	}
}

func TestMultipleRules(t *testing.T) {
	dir := t.TempDir()
	log_path := filepath.Join(dir, "app.log")

	write_log(t, log_path, "2026-03-22 10:00:00 INFO starting")

	tail := Open(log_path)
	tail.SetInterval(50 * time.Millisecond)

	var mu sync.Mutex
	errors := 0
	warnings := 0

	tail.MustOn(`ERROR`, func(m Match) {
		mu.Lock()
		errors++
		mu.Unlock()
	})

	tail.MustOn(`WARN`, func(m Match) {
		mu.Lock()
		warnings++
		mu.Unlock()
	})

	go func() {
		time.Sleep(150 * time.Millisecond)
		write_log(t, log_path, "2026-03-22 10:00:01 ERROR bad thing")
		write_log(t, log_path, "2026-03-22 10:00:02 WARN not great")
		write_log(t, log_path, "2026-03-22 10:00:03 ERROR worse thing")
		time.Sleep(300 * time.Millisecond)
		tail.Close()
	}()

	if err := tail.Run(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if errors != 2 {
		t.Errorf("expected 2 errors, got %d", errors)
	}
	if warnings != 1 {
		t.Errorf("expected 1 warning, got %d", warnings)
	}
}

func TestNoTimestamp(t *testing.T) {
	dir := t.TempDir()
	log_path := filepath.Join(dir, "app.log")

	// Log file without timestamps.
	write_log(t, log_path, "plain log line one")

	tail := Open(log_path)
	tail.SetInterval(50 * time.Millisecond)
	tail.FromStart()

	var mu sync.Mutex
	var matches []Match

	tail.MustOn(`plain (.+)`, func(m Match) {
		mu.Lock()
		matches = append(matches, m)
		mu.Unlock()
	})

	go func() {
		time.Sleep(150 * time.Millisecond)
		write_log(t, log_path, "plain log line two")
		time.Sleep(300 * time.Millisecond)
		tail.Close()
	}()

	if err := tail.Run(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Time.IsZero() == false {
		t.Error("expected zero time for lines without timestamps")
	}
	if matches[0].Text != "plain log line one" {
		t.Errorf("unexpected text: %s", matches[0].Text)
	}
}

func TestTimestampStripping(t *testing.T) {
	dir := t.TempDir()
	log_path := filepath.Join(dir, "app.log")

	write_log(t, log_path, "2026-03-22 10:00:00 ERROR something broke")

	tail := Open(log_path)
	tail.SetInterval(50 * time.Millisecond)
	tail.FromStart()

	var mu sync.Mutex
	var got Match

	tail.MustOn(`ERROR`, func(m Match) {
		mu.Lock()
		got = m
		mu.Unlock()
	})

	go func() {
		time.Sleep(300 * time.Millisecond)
		tail.Close()
	}()

	if err := tail.Run(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()

	if got.Full != "2026-03-22 10:00:00 ERROR something broke" {
		t.Errorf("unexpected Full: %s", got.Full)
	}
	if got.Text != "ERROR something broke" {
		t.Errorf("unexpected Text: %s", got.Text)
	}
	expected := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	if !got.Time.Equal(expected) {
		t.Errorf("unexpected Time: %v", got.Time)
	}
}
