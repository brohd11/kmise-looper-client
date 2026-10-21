package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/brohd11/kmise-looper-client/device"
	"github.com/brohd11/kmise-looper-client/looper"
	"github.com/brohd11/kmise-looper-client/protocol"
)

var (
	exportAll    bool
	exportOut    string
	exportPrefix string
)

var exportCmd = &cobra.Command{
	Use:   "export [SLOTS...]",
	Short: "Export slots to 24-bit WAV files. SLOTS: e.g. 3  3,7  10-15.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !exportAll {
			return errors.New("give slot numbers or --all")
		}
		var wanted []int
		if !exportAll {
			var err error
			if wanted, err = looper.ParseSlots(args); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(exportOut, 0o755); err != nil {
			return err
		}
		d, err := device.Open()
		if err != nil {
			return err
		}
		defer d.Close()
		slots, err := d.ListSlots()
		if err != nil {
			return err
		}
		listing := looper.SlotMap(slots)
		if exportAll {
			for _, s := range slots {
				if !s.Empty() {
					wanted = append(wanted, s.Index)
				}
			}
		}
		if len(wanted) == 0 {
			fmt.Println("Nothing to export.")
			return nil
		}

		// Ctrl-C stops between packets so the pedal is drained, not left mid-transfer.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		for _, i := range wanted {
			if listing[i].Empty() {
				fmt.Printf("  slot %2d: empty, skipped\n", i)
				continue
			}
			path := filepath.Join(exportOut, looper.ExportName(exportPrefix, i))
			update, finish := progressBar(fmt.Sprintf("  slot %2d", i))
			n, err := looper.ExportSlot(ctx, d, i, path, update)
			finish()
			if err != nil {
				return err
			}
			secs := float64(n) / protocol.FrameBytes / protocol.SampleRate
			fmt.Printf("  -> %s  (%s)\n", path, looper.FormatTime(secs))
		}
		return nil
	},
}

func init() {
	f := exportCmd.Flags()
	f.BoolVar(&exportAll, "all", false, "Export every non-empty slot.")
	f.StringVarP(&exportOut, "out", "o", ".", "Output directory.")
	f.StringVar(&exportPrefix, "prefix", "slot", "File name prefix.")
	rootCmd.AddCommand(exportCmd)
}
