package linkrot

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
)

// Summary counts results by status.
type Summary struct {
	Total, OK, Broken, Skipped int
}

// Summarize tallies a result set.
func Summarize(results []Result) Summary {
	s := Summary{Total: len(results)}
	for _, r := range results {
		switch r.Status {
		case StatusOK:
			s.OK++
		case StatusBroken:
			s.Broken++
		case StatusSkipped:
			s.Skipped++
		}
	}
	return s
}

// Table renders results as an aligned text table (worst-first). When onlyBroken is set, only
// broken rows are shown.
func Table(results []Result, onlyBroken bool) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tCODE\tURL\tREFERENCED BY")
	for _, r := range results {
		if onlyBroken && r.Status != StatusBroken {
			continue
		}
		code := ""
		if r.StatusCode > 0 {
			code = fmt.Sprintf("%d", r.StatusCode)
		}
		detail := r.URL
		if r.Error != "" {
			detail += "  — " + r.Error
		}
		ref := ""
		if len(r.Refs) > 0 {
			ref = r.Refs[0]
			if len(r.Refs) > 1 {
				ref += fmt.Sprintf(" (+%d more)", len(r.Refs)-1)
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Status, code, detail, ref)
	}
	_ = w.Flush()
	return b.String()
}

// JSON renders results as indented JSON.
func JSON(results []Result) string {
	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(b)
}
