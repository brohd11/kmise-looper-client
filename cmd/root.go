// Package cmd is the `kmise` command-line interface: a thin layer over package looper.
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brohd11/kmise-looper-client/device"
	"github.com/brohd11/kmise-looper-client/looper"
)

var _ looper.Pedal = (*device.Device)(nil)

// version is stamped by the makefile's LDFLAGS.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:           "kmise",
	Short:         "Talk to the YUER XS04 LOOP&DRUM (KMISE / lotmusic / Flamma / Donner) looper.",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// confirm asks a yes/no question on stdin; anything but y/yes is no.
func confirm(question string) bool {
	fmt.Printf("%s [y/N]: ", question)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
