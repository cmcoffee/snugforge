package xpect

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCommandStartAndClose(t *testing.T) {
	s, err := Command("echo", "hello")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	m, err := s.Expect("hello")
	if err != nil {
		t.Fatalf("Expect failed: %v", err)
	}
	if m.Full != "hello" {
		t.Errorf("expected Full='hello', got %q", m.Full)
	}
}

func TestExpectWithGroups(t *testing.T) {
	s, err := Command("echo", "name: Alice age: 30")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	m, err := s.Expect(`name: (\w+) age: (\d+)`)
	if err != nil {
		t.Fatalf("Expect failed: %v", err)
	}
	if len(m.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(m.Groups))
	}
	if m.Groups[0] != "Alice" {
		t.Errorf("expected group[0]='Alice', got %q", m.Groups[0])
	}
	if m.Groups[1] != "30" {
		t.Errorf("expected group[1]='30', got %q", m.Groups[1])
	}
}

func TestExpectTimeout(t *testing.T) {
	s, err := Command("cat")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	_, err = s.ExpectTimeout("never_match", 200*time.Millisecond)
	if err != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
}

func TestSendAndExpect(t *testing.T) {
	s, err := Command("cat")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	if err := s.SendLine("hello world"); err != nil {
		t.Fatalf("SendLine failed: %v", err)
	}

	m, err := s.Expect("hello world")
	if err != nil {
		t.Fatalf("Expect failed: %v", err)
	}
	if !strings.Contains(m.Full, "hello world") {
		t.Errorf("expected match containing 'hello world', got %q", m.Full)
	}
}

func TestExpectEOF(t *testing.T) {
	s, err := Command("echo", "done")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	if err := s.ExpectEOF(); err != nil {
		t.Errorf("ExpectEOF failed: %v", err)
	}
}

func TestExpectBefore(t *testing.T) {
	s, err := Command("echo", "prefix:target")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	m, err := s.Expect("target")
	if err != nil {
		t.Fatalf("Expect failed: %v", err)
	}
	if !strings.Contains(m.Before, "prefix:") {
		t.Errorf("expected Before to contain 'prefix:', got %q", m.Before)
	}
}

func TestOutputAndClear(t *testing.T) {
	s, err := Command("echo", "buffered text")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	// Give the reader time to populate buffer.
	time.Sleep(100 * time.Millisecond)

	out := s.Output()
	if !strings.Contains(out, "buffered text") {
		t.Errorf("expected Output to contain 'buffered text', got %q", out)
	}

	s.Clear()
	if s.Output() != "" {
		t.Errorf("expected empty buffer after Clear, got %q", s.Output())
	}
}

func TestSetTimeout(t *testing.T) {
	s, err := Command("cat")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	s.SetTimeout(200 * time.Millisecond)
	_, err = s.Expect("never_match")
	if err != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
}

func TestCommandNotFound(t *testing.T) {
	_, err := Command("nonexistent_command_xyzzy_12345")
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestLogOutput(t *testing.T) {
	var log bytes.Buffer

	s, err := Command("echo", "visible output")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	s.Log = &log

	_, err = s.Expect("visible output")
	if err != nil {
		t.Fatalf("Expect failed: %v", err)
	}

	if !strings.Contains(log.String(), "visible output") {
		t.Errorf("expected log to contain 'visible output', got %q", log.String())
	}
}

func TestSendLog(t *testing.T) {
	var log bytes.Buffer

	s, err := Command("cat")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	s.Log = &log
	s.SendLog = true

	if err := s.SendLine("typed input"); err != nil {
		t.Fatalf("SendLine failed: %v", err)
	}

	_, err = s.Expect("typed input")
	if err != nil {
		t.Fatalf("Expect failed: %v", err)
	}

	logged := log.String()
	if !strings.Contains(logged, "typed input\n") {
		t.Errorf("expected log to contain sent input, got %q", logged)
	}
}

func TestSendMask(t *testing.T) {
	var log bytes.Buffer

	// Use a command that does not echo input back so we can verify
	// the mask replaces the sent text in the log.
	s, err := Command("sh", "-c", "read line; echo done")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	s.Log = &log
	s.SendLog = true

	s.SendMask = "****\n"
	if err := s.SendLine("secret123"); err != nil {
		t.Fatalf("SendLine failed: %v", err)
	}

	_, err = s.Expect("done")
	if err != nil {
		t.Fatalf("Expect failed: %v", err)
	}

	logged := log.String()
	if strings.Contains(logged, "secret123") {
		t.Errorf("log should not contain the secret, got %q", logged)
	}
	if !strings.Contains(logged, "****") {
		t.Errorf("log should contain mask '****', got %q", logged)
	}
}

func TestSendMaskResetsAfterUse(t *testing.T) {
	var log bytes.Buffer

	s, err := Command("cat")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	s.Log = &log
	s.SendLog = true

	// First send is masked.
	s.SendMask = "****\n"
	s.SendLine("secret")

	// Second send should not be masked.
	s.SendLine("visible")

	_, err = s.Expect("visible")
	if err != nil {
		t.Fatalf("Expect failed: %v", err)
	}

	logged := log.String()
	if !strings.Contains(logged, "visible\n") {
		t.Errorf("expected second send to appear unmasked in log, got %q", logged)
	}
}

func TestInteract(t *testing.T) {
	// Use a script that echoes back what it receives, then exits.
	s, err := Command("sh", "-c", "read line; echo got: $line; read line2; echo got: $line2")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	var output bytes.Buffer
	input := strings.NewReader("hello\nworld\n")

	err = s.Interact(input, &output)
	if err != nil {
		t.Fatalf("Interact failed: %v", err)
	}

	out := output.String()
	if !strings.Contains(out, "got: hello") {
		t.Errorf("expected interact output to contain 'got: hello', got %q", out)
	}
	if !strings.Contains(out, "got: world") {
		t.Errorf("expected interact output to contain 'got: world', got %q", out)
	}
}

func TestInteractUntil(t *testing.T) {
	// Script that prompts, echoes, then prints a marker we can match on.
	s, err := Command("sh", "-c", `echo "ready"; read line; echo "got: $line"; echo "PROMPT>"`)
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	var output bytes.Buffer
	input := strings.NewReader("test input\n")

	m, err := s.InteractUntil(input, &output, `PROMPT>`)
	if err != nil {
		t.Fatalf("InteractUntil failed: %v", err)
	}

	if m.Full != "PROMPT>" {
		t.Errorf("expected match Full='PROMPT>', got %q", m.Full)
	}

	out := output.String()
	if !strings.Contains(out, "ready") {
		t.Errorf("expected interact output to contain 'ready', got %q", out)
	}
}

func TestInteractThenExpect(t *testing.T) {
	// Verify that scripted Expect works after Interact returns.
	s, err := Command("sh", "-c", `echo "phase1"; read line; echo "phase2"`)
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
	defer s.Close()

	var output bytes.Buffer
	input := strings.NewReader("go\n")

	// InteractUntil phase1, then resume scripted control.
	_, err = s.InteractUntil(input, &output, "phase1")
	if err != nil {
		t.Fatalf("InteractUntil failed: %v", err)
	}

	// Now expect phase2 via scripted expect.
	m, err := s.Expect("phase2")
	if err != nil {
		t.Fatalf("Expect after interact failed: %v", err)
	}
	if m.Full != "phase2" {
		t.Errorf("expected 'phase2', got %q", m.Full)
	}
}

func TestCloseTwice(t *testing.T) {
	s, err := Command("echo", "test")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if err := s.Close(); err != nil {
		// Process may have already exited, which is fine.
		_ = err
	}

	if err := s.Close(); err != ErrClosed {
		t.Errorf("expected ErrClosed on second close, got %v", err)
	}
}
