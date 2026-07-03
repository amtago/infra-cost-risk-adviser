package report

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/pricing"
	"github.com/amt/tf-cost-risk/rules"
)

// TextFormatter renders a Report as human-readable terminal output.
type TextFormatter struct{}

func (f *TextFormatter) Format(r Report) ([]byte, error) {
	var b bytes.Buffer
	writeSummaryLine(&b, r)
	b.WriteByte('\n')
	writeCostTable(&b, r)
	b.WriteByte('\n')
	writeFindings(&b, r)
	return b.Bytes(), nil
}

func writeSummaryLine(b *bytes.Buffer, r Report) {
	s := r.Summary
	criticals := s.CountBySeverity[rules.SeverityCritical]
	warnings := s.CountBySeverity[rules.SeverityWarning]
	totalFindings := criticals + warnings + s.CountBySeverity[rules.SeverityInfo]

	var costPart string
	switch {
	case s.NetDeltaUSD > 0:
		costPart = fmt.Sprintf("adds $%.2f/mo", s.NetDeltaUSD)
	case s.NetDeltaUSD < 0:
		costPart = fmt.Sprintf("removes $%.2f/mo", -s.NetDeltaUSD)
	default:
		costPart = "has no net cost change"
	}

	var findingPart string
	switch {
	case totalFindings == 0:
		findingPart = "no issues found"
	case criticals > 0:
		findingPart = fmt.Sprintf("%d critical issue(s) require attention", criticals)
	default:
		findingPart = fmt.Sprintf("%d issue(s) found", totalFindings)
	}

	fmt.Fprintf(b, "This plan %s and has %s.\n", costPart, findingPart)

	if s.UnknownCount > 0 {
		fmt.Fprintf(b, "Note: %d resource(s) have unknown cost (usage-based pricing; see table below).\n", s.UnknownCount)
	}
}

func writeCostTable(b *bytes.Buffer, r Report) {
	if len(r.Estimates) == 0 {
		fmt.Fprintln(b, "Cost table: no resources with pricing data.")
		return
	}

	estimates := make([]pricing.Estimate, len(r.Estimates))
	copy(estimates, r.Estimates)
	sort.Slice(estimates, func(i, j int) bool {
		return estimates[i].ResourceAddress < estimates[j].ResourceAddress
	})

	fmt.Fprintln(b, "Cost breakdown ($/mo):")
	fmt.Fprintf(b, "  %-55s %-10s %s\n", "Resource", "Change", "$/mo")
	fmt.Fprintf(b, "  %s\n", repeat("-", 80))

	for _, e := range estimates {
		if e.Unknown {
			fmt.Fprintf(b, "  %-55s %-10s %s\n", truncate(e.ResourceAddress, 54), changeSymbol(e.ChangeType), "unknown")
		} else {
			fmt.Fprintf(b, "  %-55s %-10s $%.2f\n", truncate(e.ResourceAddress, 54), changeSymbol(e.ChangeType), e.MonthlyCostUSD)
		}
	}

	fmt.Fprintf(b, "  %s\n", repeat("-", 80))
	if r.Summary.TotalRemovedUSD > 0 {
		fmt.Fprintf(b, "  %-67s $%.2f added\n", "", r.Summary.TotalAddedUSD)
		fmt.Fprintf(b, "  %-67s-$%.2f removed\n", "", r.Summary.TotalRemovedUSD)
	}
	fmt.Fprintf(b, "  %-67s $%.2f/mo net\n", "", r.Summary.NetDeltaUSD)
}

func writeFindings(b *bytes.Buffer, r Report) {
	if len(r.Findings) == 0 {
		fmt.Fprintln(b, "Findings: none.")
		return
	}

	fmt.Fprintln(b, "Findings (grouped by severity):")
	for _, sev := range []rules.Severity{rules.SeverityCritical, rules.SeverityWarning, rules.SeverityInfo} {
		var group []rules.Finding
		for _, f := range r.Findings {
			if f.Severity == sev {
				group = append(group, f)
			}
		}
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(b, "\n  [%s]\n", sevLabel(sev))
		for _, f := range group {
			fmt.Fprintf(b, "  • [%s] %s\n", f.Category, f.Explanation)
		}
	}
}

func sevLabel(s rules.Severity) string {
	switch s {
	case rules.SeverityCritical:
		return "CRITICAL"
	case rules.SeverityWarning:
		return "WARNING"
	case rules.SeverityInfo:
		return "INFO"
	}
	return string(s)
}

func changeSymbol(ct parser.ChangeType) string {
	switch ct {
	case parser.ChangeCreate:
		return "+ create"
	case parser.ChangeDelete:
		return "- delete"
	case parser.ChangeUpdate:
		return "~ update"
	case parser.ChangeReplace:
		return "± replace"
	}
	return ""
}

func repeat(s string, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = s[0]
	}
	return string(b)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
