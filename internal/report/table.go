// Package report formats check results for the terminal.
package report

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/sarahhghazali/healthcheck/internal/checker"
)

// PrintTable writes results as an aligned table.
func PrintTable(w io.Writer, results []checker.Result) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tNAME\tCODE\tLATENCY\tDETAILS")

	for _, r := range results {
		status := "UP"
		if !r.Up {
			status = "DOWN"
		}

		code := "-"
		if r.StatusCode != 0 {
			code = fmt.Sprint(r.StatusCode)
		}

		details := ""
		if r.Err != nil {
			details = r.Err.Error()
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			status, r.Name, code, r.Latency.Round(time.Millisecond), details)
	}
	tw.Flush()
}

// Summary returns a one-line count like "3/4 services up".
func Summary(results []checker.Result) string {
	up := 0
	for _, r := range results {
		if r.Up {
			up++
		}
	}
	return fmt.Sprintf("%d/%d services up", up, len(results))
}
