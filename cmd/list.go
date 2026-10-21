package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/brohd11/kmise-looper-client/device"
	"github.com/brohd11/kmise-looper-client/looper"
)

var listAll bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded slots.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := device.Open()
		if err != nil {
			return err
		}
		slots, err := d.ListSlots()
		d.Close()
		if err != nil {
			return err
		}
		shown := 0
		for _, s := range slots {
			if s.Empty() && !listAll {
				continue
			}
			shown++
			size := "   empty"
			if !s.Empty() {
				size = fmt.Sprintf("%6.1f MB", float64(s.PCMBytes())/1e6)
			}
			fmt.Printf("  %2d  %9s  %s\n", s.Index, looper.FormatTime(s.Seconds()), size)
		}
		if shown == 0 {
			fmt.Println("No recordings on the device.")
		}
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVar(&listAll, "all", false, "Include empty slots.")
	rootCmd.AddCommand(listCmd)
}
