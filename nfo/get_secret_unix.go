//go:build !windows
// +build !windows

package nfo

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/term"
)

// GetSecret prompts the user for a secret string, displaying * for each
// character typed. Supports backspace to delete characters.
func GetSecret(prompt string) string {
	unesc := Defer(getEscape())
	defer unesc()

	fmt.Printf("%s", prompt)

	term.MakeRaw(int(syscall.Stdin))

	buf := make([]byte, 1)
	var password []byte

	for {
		n, err := os.Stdin.Read(buf)
		if n == 0 || err != nil {
			break
		}
		ch := buf[0]
		switch {
		case ch == '\n' || ch == '\r':
			fmt.Printf("\n")
			return cleanInput(string(password))
		case ch == 3: // Ctrl+C
			fmt.Printf("\n")
			signalChan <- syscall.SIGINT
			return ""
		case ch == 127 || ch == 8: // Backspace or Delete
			if len(password) > 0 {
				password = password[:len(password)-1]
				fmt.Printf("\b \b")
			}
		case ch >= 32: // Printable characters
			password = append(password, ch)
			fmt.Printf("*")
		}
	}
	fmt.Printf("\n")
	return cleanInput(string(password))
}
