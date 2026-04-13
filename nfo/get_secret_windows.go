//go:build windows
// +build windows

package nfo

import (
	"fmt"
	"syscall"

	"golang.org/x/term"
)

// GetSecret prompts the user for a secret string, displaying * for each
// character typed. Supports backspace to delete characters.
func GetSecret(prompt string) string {
	fmt.Printf(prompt)

	// On Windows, term.ReadPassword handles echo suppression but
	// doesn't show asterisks. Use the console handle directly.
	handle := syscall.Stdin
	oldState, err := term.GetState(int(handle))
	if err != nil {
		// Fallback: read without masking.
		resp, _ := term.ReadPassword(int(handle))
		fmt.Printf("\n")
		return cleanInput(string(resp))
	}
	defer term.Restore(int(handle), oldState)

	term.MakeRaw(int(handle))

	buf := make([]byte, 1)
	var password []byte
	fd := syscall.Handle(handle)

	for {
		var n uint32
		err := syscall.ReadFile(fd, buf, &n, nil)
		if err != nil || n == 0 {
			break
		}
		ch := buf[0]
		switch {
		case ch == '\r' || ch == '\n':
			fmt.Printf("\n")
			return cleanInput(string(password))
		case ch == 3: // Ctrl+C
			fmt.Printf("\n")
			return ""
		case ch == 8: // Backspace
			if len(password) > 0 {
				password = password[:len(password)-1]
				fmt.Printf("\b \b")
			}
		case ch >= 32:
			password = append(password, ch)
			fmt.Printf("*")
		}
	}
	fmt.Printf("\n")
	return cleanInput(string(password))
}
