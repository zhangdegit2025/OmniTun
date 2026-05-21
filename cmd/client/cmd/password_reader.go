package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func readPassword() (string, error) {
	stdin := windows.Handle(os.Stdin.Fd())

	var originalMode uint32
	if err := windows.GetConsoleMode(stdin, &originalMode); err != nil {
		reader := bufio.NewReader(os.Stdin)
		s, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(s), nil
	}
	defer windows.SetConsoleMode(stdin, originalMode)

	newMode := originalMode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT)
	newMode |= windows.ENABLE_PROCESSED_INPUT
	if err := windows.SetConsoleMode(stdin, newMode); err != nil {
		reader := bufio.NewReader(os.Stdin)
		s, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(s), nil
	}

	var password []byte
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		if buf[0] == '\r' || buf[0] == '\n' {
			break
		}
		if buf[0] == '\b' || buf[0] == 127 {
			if len(password) > 0 {
				password = password[:len(password)-1]
				fmt.Fprint(os.Stderr, "\b \b")
			}
			continue
		}
		password = append(password, buf[0])
		fmt.Fprint(os.Stderr, "*")
	}

	if len(password) == 0 {
		return "", fmt.Errorf("empty password")
	}
	return string(password), nil
}
