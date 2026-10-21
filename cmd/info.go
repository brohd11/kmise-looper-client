package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/brohd11/kmise-looper-client/device"
	"github.com/brohd11/kmise-looper-client/looper"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show firmware version and slot usage.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := device.Open()
		if err != nil {
			return err
		}
		defer d.Close()
		version, slots, err := looper.Info(d)
		if err != nil {
			return err
		}
		used, total := 0, 0.0
		for _, s := range slots {
			if !s.Empty() {
				used++
				total += s.Seconds()
			}
		}
		fmt.Printf("Device:   XS04 LOOP&DRUM (version %q)\n", version)
		fmt.Printf("Slots:    %d of %d used\n", used, len(slots))
		fmt.Printf("Audio:    %s total\n", looper.FormatTime(total))
		return nil
	},
}

func init() { rootCmd.AddCommand(infoCmd) }
