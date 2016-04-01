package bot

import (
	"io"
	"text/tabwriter"
)

func TabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 5, 0, 1, ' ', tabwriter.AlignRight)
}
