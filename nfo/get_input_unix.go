//go:build !windows
// +build !windows

package nfo

import (
	"fmt"
	"io"
	"os"
	"syscall"

	"golang.org/x/term"
)

// GetInput prompts the user for input and returns the entered string.
func GetInput(prompt string) string {
	unesc := Defer(getEscape())
	defer unesc()

	fmt.Printf(prompt)

	term.MakeRaw(int(syscall.Stdin))

	var (
		str string
		err error
	)

	for {
		t := term.NewTerminal(os.Stdin, "")
		str, err = t.ReadLine()
		if err == io.EOF {
			signalChan <- syscall.SIGINT
			continue
		}
		break
	}
	return cleanInput(str)
}
