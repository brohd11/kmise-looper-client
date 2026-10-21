package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/brohd11/kmise-looper-client/device"
	"github.com/brohd11/kmise-looper-client/looper"
)

var importForce bool

var importCmd = &cobra.Command{
	Use:   "import SLOT FILE",
	Short: "Import an audio FILE into SLOT.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		slot, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid slot %q", args[0])
		}
		if _, err := looper.ParseSlots([]string{args[0]}); err != nil {
			return err
		}
		pcm, err := looper.LoadForImport(args[1])
		if err != nil {
			return err
		}
		d, err := device.Open()
		if err != nil {
			return err
		}
		defer d.Close()
		update, finish := progressBar(fmt.Sprintf("  slot %2d", slot))
		after, err := looper.ImportPCM(d, slot, pcm, importForce, update)
		finish()
		var occupied *looper.SlotOccupiedError
		if errors.As(err, &occupied) {
			return fmt.Errorf("%v; use --force to overwrite", err)
		}
		if err != nil {
			return err
		}
		fmt.Printf("  imported %s -> slot %d (%s on device)\n",
			filepath.Base(args[1]), slot, looper.FormatTime(after.Seconds()))
		return nil
	},
}

func init() {
	importCmd.Flags().BoolVar(&importForce, "force", false, "Overwrite a non-empty slot.")
	rootCmd.AddCommand(importCmd)
}
