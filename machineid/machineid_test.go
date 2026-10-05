package machineid

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The ID is derived per application and stable; the raw identity is never
// what comes back.
func TestIDIsPerAppAndStable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads Linux machine-id files")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "machine-id")
	os.WriteFile(p, []byte("0123456789abcdef0123456789abcdef\n"), 0o600)
	prev := linuxPaths
	linuxPaths = []string{p}
	t.Cleanup(func() { linuxPaths = prev })

	a, err := ID("gohort")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ID("gohort")
	c, _ := ID("kitebroker")
	if string(a) != string(b) || string(a) == string(c) || string(a) == "0123456789abcdef0123456789abcdef" || len(a) != 64 {
		t.Fatalf("a=%s b=%s c=%s", a, b, c)
	}
}

// An empty, placeholder or all-zero identity is no identity, and the next
// source is tried.
func TestNoUsableID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads Linux machine-id files")
	}
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	os.WriteFile(empty, []byte("\n"), 0o600)
	zero := filepath.Join(dir, "zero")
	os.WriteFile(zero, []byte("00000000000000000000000000000000"), 0o600)
	prev := linuxPaths
	t.Cleanup(func() { linuxPaths = prev })

	linuxPaths = []string{empty, zero, filepath.Join(dir, "missing")}
	if _, err := ID("x"); err != ErrNoID {
		t.Fatalf("no usable source: %v", err)
	}
	good := filepath.Join(dir, "good")
	os.WriteFile(good, []byte("feedfacefeedfacefeedfacefeedface"), 0o600)
	linuxPaths = []string{empty, good}
	if _, err := ID("x"); err != nil {
		t.Fatalf("fell through to the second source: %v", err)
	}
	if _, err := usable("uninitialized"); err != ErrNoID {
		t.Fatal("the pre-boot placeholder counted as an ID")
	}
}

func TestParsers(t *testing.T) {
	ioreg := `+-o IOPlatformExpertDevice  <class IOPlatformExpertDevice>
    {
      "IOPlatformSerialNumber" = "C02XXXX"
      "IOPlatformUUID" = "564D2A1B-1234-5678-9ABC-DEF012345678"
    }`
	if got := parseIOReg(ioreg); got != "564D2A1B-1234-5678-9ABC-DEF012345678" {
		t.Fatalf("ioreg: %q", got)
	}
	reg := "\r\nHKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Cryptography\r\n    MachineGuid    REG_SZ    3f2504e0-4f89-11d3-9a0c-0305e82c3301\r\n"
	if got := parseRegQuery(reg); got != "3f2504e0-4f89-11d3-9a0c-0305e82c3301" {
		t.Fatalf("reg: %q", got)
	}
}
