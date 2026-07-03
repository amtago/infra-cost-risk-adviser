// Package report renders findings and cost estimates to output formats.
package report

import (
	"github.com/amt/tf-cost-risk/parser"
	"github.com/amt/tf-cost-risk/pricing"
	"github.com/amt/tf-cost-risk/rules"
)

// Report is the shared data model passed to all formatters.
type Report struct {
	Estimates []pricing.Estimate
	Findings  []rules.Finding
	Summary   Summary
}

// Summary holds pre-computed aggregates derived from Estimates and Findings.
type Summary struct {
	TotalAddedUSD   float64
	TotalRemovedUSD float64
	NetDeltaUSD     float64
	UnknownCount    int
	// CountBySeverity maps severity → number of findings at that severity.
	CountBySeverity map[rules.Severity]int
}

// Formatter renders a Report to a target output format.
type Formatter interface {
	Format(r Report) ([]byte, error)
}

// Build constructs a Report from raw pipeline outputs, computing the Summary.
func Build(estimates []pricing.Estimate, findings []rules.Finding) Report {
	summary := Summary{
		CountBySeverity: map[rules.Severity]int{},
	}
	for _, e := range estimates {
		if e.Unknown {
			summary.UnknownCount++
			continue
		}
		if e.ChangeType == parser.ChangeDelete {
			summary.TotalRemovedUSD += e.MonthlyCostUSD
		} else {
			summary.TotalAddedUSD += e.MonthlyCostUSD
		}
	}
	summary.NetDeltaUSD = summary.TotalAddedUSD - summary.TotalRemovedUSD

	for _, f := range findings {
		summary.CountBySeverity[f.Severity]++
	}

	return Report{
		Estimates: estimates,
		Findings:  findings,
		Summary:   summary,
	}
}
