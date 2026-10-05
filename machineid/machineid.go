// Package machineid identifies the machine a program runs on, for keys that
// should open only there.
//
// The source is the operating system's own install identity, not hardware
// enumeration: a MAC address changes with the order interfaces are listed
// (containers, VMs, USB adapters coming and going), and a key derived from
// it stops opening what it locked. What it reads:
//
//	Linux, WSL2: /etc/machine-id, else /var/lib/dbus/machine-id
//	macOS:       IOPlatformUUID (ioreg)
//	Windows:     HKLM\SOFTWARE\Microsoft\Cryptography\MachineGuid
//
// ID never returns the raw identity. It returns a hash of it with the
// caller's application name, as systemd asks of anyone using machine-id:
// the raw value is not a secret and is shared by every program on the box,
// while each application's derived ID is its own.
package machineid

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// ErrNoID is returned when the machine has no usable identity: a container
// with an empty /etc/machine-id, a platform with none, or one that could not
// be read.
var ErrNoID = errors.New("this machine has no usable machine ID")

// ID is this machine's identity for app: a hex SHA-256 of the machine's ID
// and app, the same on every call on this install of the operating system.
func ID(app string) ([]byte, error) {
	raw, err := rawID()
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(raw + "|" + app))
	return []byte(hex.EncodeToString(sum[:])), nil
}

// linuxPaths are read in order; a test points them elsewhere.
var linuxPaths = []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}

func rawID() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err != nil {
			return "", ErrNoID
		}
		return usable(parseIOReg(string(out)))
	case "windows":
		out, err := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").Output()
		if err != nil {
			return "", ErrNoID
		}
		return usable(parseRegQuery(string(out)))
	default:
		for _, p := range linuxPaths {
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			if id, err := usable(string(b)); err == nil {
				return id, nil
			}
		}
		return "", ErrNoID
	}
}

// usable is id trimmed, or ErrNoID when it is empty, the placeholder systemd
// writes before first boot, or all zeros.
func usable(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || id == "uninitialized" || strings.Trim(id, "0-") == "" {
		return "", ErrNoID
	}
	return id, nil
}

var ioregUUID = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)

// parseIOReg pulls IOPlatformUUID out of ioreg's output.
func parseIOReg(out string) string {
	if m := ioregUUID.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

// parseRegQuery pulls MachineGuid's value out of reg query's output.
func parseRegQuery(out string) string {
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && strings.EqualFold(f[0], "MachineGuid") {
			return f[len(f)-1]
		}
	}
	return ""
}
