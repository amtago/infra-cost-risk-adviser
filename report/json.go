package report

import "encoding/json"

// JSONFormatter renders a Report as machine-readable JSON.
type JSONFormatter struct{}

type jsonReport struct {
	Summary   jsonSummary   `json:"summary"`
	Costs     []jsonCost    `json:"costs"`
	Findings  []jsonFinding `json:"findings"`
}

type jsonSummary struct {
	TotalAddedUSD   float64        `json:"total_added_usd"`
	TotalRemovedUSD float64        `json:"total_removed_usd"`
	NetDeltaUSD     float64        `json:"net_delta_usd"`
	UnknownCount    int            `json:"unknown_count"`
	FindingCounts   map[string]int `json:"finding_counts"`
}

type jsonCost struct {
	ResourceAddress string  `json:"resource_address"`
	ChangeType      string  `json:"change_type"`
	MonthlyCostUSD  float64 `json:"monthly_cost_usd,omitempty"`
	Unknown         bool    `json:"unknown,omitempty"`
}

type jsonFinding struct {
	Severity        string `json:"severity"`
	Category        string `json:"category"`
	ResourceAddress string `json:"resource_address"`
	Explanation     string `json:"explanation"`
}

func (f *JSONFormatter) Format(r Report) ([]byte, error) {
	counts := map[string]int{}
	for sev, n := range r.Summary.CountBySeverity {
		counts[string(sev)] = n
	}

	costs := make([]jsonCost, 0, len(r.Estimates))
	for _, e := range r.Estimates {
		costs = append(costs, jsonCost{
			ResourceAddress: e.ResourceAddress,
			ChangeType:      string(e.ChangeType),
			MonthlyCostUSD:  e.MonthlyCostUSD,
			Unknown:         e.Unknown,
		})
	}

	findings := make([]jsonFinding, 0, len(r.Findings))
	for _, fin := range r.Findings {
		findings = append(findings, jsonFinding{
			Severity:        string(fin.Severity),
			Category:        string(fin.Category),
			ResourceAddress: fin.ResourceAddress,
			Explanation:     fin.Explanation,
		})
	}

	out := jsonReport{
		Summary: jsonSummary{
			TotalAddedUSD:   r.Summary.TotalAddedUSD,
			TotalRemovedUSD: r.Summary.TotalRemovedUSD,
			NetDeltaUSD:     r.Summary.NetDeltaUSD,
			UnknownCount:    r.Summary.UnknownCount,
			FindingCounts:   counts,
		},
		Costs:    costs,
		Findings: findings,
	}

	return json.MarshalIndent(out, "", "  ")
}
