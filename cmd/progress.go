package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/brohd11/kmise-looper-client/device"
)

const barWidth = 30

func stderrIsTerminal() bool {
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// progressBar draws a single-line bar on stderr, starting at the first update so nothing
// shows if the operation fails before transferring. finish ends the line. When stderr is not
// a terminal only the label is printed, as redraws would pile up in logs.
func progressBar(label string) (update device.Progress, finish func()) {
	last := -1
	if !stderrIsTerminal() {
		update = func(int, int) {
			if last < 0 {
				last = 0
				fmt.Fprintln(os.Stderr, label)
			}
		}
		return update, func() {}
	}
	update = func(done, total int) {
		pct := 100
		if total > 0 {
			pct = done * 100 / total
		}
		if pct == last {
			return
		}
		last = pct
		fill := pct * barWidth / 100
		fmt.Fprintf(os.Stderr, "\r%s  [%s%s]  %3d%%", label,
			strings.Repeat("#", fill), strings.Repeat("-", barWidth-fill), pct)
	}
	finish = func() {
		if last >= 0 {
			fmt.Fprintln(os.Stderr)
		}
	}
	return update, finish
}
