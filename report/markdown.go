package report

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/pricing"
	"github.com/amt/tf-cost-risk/rules"
)

// MarkdownFormatter renders a Report as GitHub-flavoured Markdown.
// Suitable for PR comments, GitHub Step Summaries, and wikis.
type MarkdownFormatter struct{}

func (f *MarkdownFormatter) Format(r Report) ([]byte, error) {
	var b bytes.Buffer
	writeMDSummaryLine(&b, r)
	b.WriteByte('\n')
	writeMDCostTable(&b, r)
	b.WriteByte('\n')
	writeMDFindings(&b, r)
	return b.Bytes(), nil
}

func writeMDSummaryLine(b *bytes.Buffer, r Report) {
	s := r.Summary
	criticals := s.CountBySeverity[rules.SeverityCritical]
	warnings := s.CountBySeverity[rules.SeverityWarning]
	totalFindings := criticals + warnings + s.CountBySeverity[rules.SeverityInfo]

	var costPart string
	switch {
	case s.NetDeltaUSD > 0:
		costPart = fmt.Sprintf("adds **$%.2f/mo**", s.NetDeltaUSD)
	case s.NetDeltaUSD < 0:
		costPart = fmt.Sprintf("removes **$%.2f/mo**", -s.NetDeltaUSD)
	default:
		costPart = "has **no net cost change**"
	}

	var findingPart string
	switch {
	case totalFindings == 0:
		findingPart = "✅ no issues found"
	case criticals > 0:
		findingPart = fmt.Sprintf("🔴 **%d critical issue(s)** require attention", criticals)
	default:
		findingPart = fmt.Sprintf("⚠️ **%d issue(s)** found", totalFindings)
	}

	fmt.Fprintf(b, "This plan %s and has %s.\n", costPart, findingPart)

	if s.UnknownCount > 0 {
		fmt.Fprintf(b, "\n> ℹ️ %d resource(s) have unknown cost (usage-based pricing).\n", s.UnknownCount)
	}
}

func writeMDCostTable(b *bytes.Buffer, r Report) {
	if len(r.Estimates) == 0 {
		fmt.Fprintln(b, "_No resources with pricing data._")
		return
	}

	estimates := make([]pricing.Estimate, len(r.Estimates))
	copy(estimates, r.Estimates)
	sort.Slice(estimates, func(i, j int) bool {
		return estimates[i].ResourceAddress < estimates[j].ResourceAddress
	})

	fmt.Fprintln(b, "### Cost breakdown ($/mo)")
	fmt.Fprintln(b, "")
	fmt.Fprintln(b, "| Resource | Change | $/mo |")
	fmt.Fprintln(b, "|---|---|---:|")

	for _, e := range estimates {
		sym := mdChangeSymbol(e.ChangeType)
		if e.Unknown {
			fmt.Fprintf(b, "| `%s` | %s | _unknown_ |\n", e.ResourceAddress, sym)
		} else {
			fmt.Fprintf(b, "| `%s` | %s | $%.2f |\n", e.ResourceAddress, sym, e.MonthlyCostUSD)
		}
	}

	fmt.Fprintln(b, "")
	if r.Summary.TotalRemovedUSD > 0 {
		fmt.Fprintf(b, "**Added:** $%.2f &nbsp; **Removed:** −$%.2f &nbsp; **Net:** $%.2f/mo\n",
			r.Summary.TotalAddedUSD, r.Summary.TotalRemovedUSD, r.Summary.NetDeltaUSD)
	} else {
		fmt.Fprintf(b, "**Net:** $%.2f/mo\n", r.Summary.NetDeltaUSD)
	}
}

func writeMDFindings(b *bytes.Buffer, r Report) {
	if len(r.Findings) == 0 {
		fmt.Fprintln(b, "### Findings\n\n_None._")
		return
	}

	fmt.Fprintln(b, "### Findings")

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
		fmt.Fprintf(b, "\n#### %s %s\n\n", mdSevIcon(sev), mdSevLabel(sev))
		for _, f := range group {
			fmt.Fprintf(b, "- **[%s]** %s\n", f.Category, f.Explanation)
		}
	}
}

func mdSevIcon(s rules.Severity) string {
	switch s {
	case rules.SeverityCritical:
		return "🔴"
	case rules.SeverityWarning:
		return "⚠️"
	case rules.SeverityInfo:
		return "ℹ️"
	}
	return ""
}

func mdSevLabel(s rules.Severity) string {
	switch s {
	case rules.SeverityCritical:
		return "Critical"
	case rules.SeverityWarning:
		return "Warning"
	case rules.SeverityInfo:
		return "Info"
	}
	return string(s)
}

func mdChangeSymbol(ct parser.ChangeType) string {
	switch ct {
	case parser.ChangeCreate:
		return "➕ create"
	case parser.ChangeDelete:
		return "🗑️ delete"
	case parser.ChangeUpdate:
		return "✏️ update"
	case parser.ChangeReplace:
		return "🔄 replace"
	}
	return string(ct)
}
