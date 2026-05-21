package main

import (
	"fmt"
	"os"

	"github.com/omnitun/omnitun/cmd/client/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31merror:\033[0m %v\n", err)
		os.Exit(1)
	}
}
