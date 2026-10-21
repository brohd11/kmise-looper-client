package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brohd11/kmise-looper-client/device"
	"github.com/brohd11/kmise-looper-client/looper"
	"github.com/brohd11/kmise-looper-client/protocol"
)

var (
	deleteAll bool
	deleteYes bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete [SLOTS...]",
	Short: "Delete slots. SLOTS: e.g. 3  3,7  10-15.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !deleteAll {
			return errors.New("give slot numbers or --all")
		}
		var targets []int
		what := "ALL slots"
		if deleteAll {
			for i := 0; i < protocol.NumSlots; i++ {
				targets = append(targets, i)
			}
		} else {
			var err error
			if targets, err = looper.ParseSlots(args); err != nil {
				return err
			}
			names := make([]string, len(targets))
			for i, s := range targets {
				names[i] = strconv.Itoa(s)
			}
			what = "slot(s) " + strings.Join(names, ", ")
		}
		if !deleteYes && !confirm(fmt.Sprintf("Permanently delete %s?", what)) {
			return errors.New("aborted")
		}
		d, err := device.Open()
		if err != nil {
			return err
		}
		defer d.Close()
		if err := looper.Delete(d, targets); err != nil {
			return err
		}
		fmt.Printf("Deleted %s.\n", what)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVar(&deleteAll, "all", false, "Delete EVERY slot.")
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "Don't ask for confirmation.")
	rootCmd.AddCommand(deleteCmd)
}
