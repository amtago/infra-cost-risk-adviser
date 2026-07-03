// Package main is the entry point for the tfx CLI.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	awsnorm "github.com/amt/tf-cost-risk/normalizer/providers/aws"
	"github.com/amt/tf-cost-risk/parser"
	awspricing "github.com/amt/tf-cost-risk/pricing/providers/aws"
	"github.com/amt/tf-cost-risk/report"
	awsrules "github.com/amt/tf-cost-risk/rules/providers/aws"

	"github.com/amt/tf-cost-risk/normalizer"
	"github.com/amt/tf-cost-risk/pricing"
	"github.com/amt/tf-cost-risk/rules"
)

const version = "0.1.0"

const usage = `tfx — Terraform plan cost & risk analyzer

Usage:
  tfx analyze <plan.json> [flags]
  tfx version

Flags:
  --format text|json   Output format (default: text)
  --region <region>    AWS region to use when not inferable from the plan (default: us-east-1)
  --help               Show this help

Examples:
  terraform show -json tfplan.binary > plan.json
  tfx analyze plan.json
  tfx analyze plan.json --format json --region eu-west-1
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "analyze":
		runAnalyze(os.Args[2:])
	case "version":
		fmt.Printf("tfx v%s\n", version)
	case "--help", "-h", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

func runAnalyze(args []string) {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	formatFlag := fs.String("format", "text", "Output format: text or json")
	regionFlag := fs.String("region", "us-east-1", "AWS region for pricing lookups")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	// Allow: analyze plan.json --format json  (file before flags)
	// and:   analyze --format json plan.json  (flags before file)
	// Separate the plan file from flag args so flag.Parse only sees flags.
	var planFile string
	var flagArgs []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && planFile == "" {
			planFile = a
		} else {
			flagArgs = append(flagArgs, a)
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(1)
	}

	if planFile == "" {
		fmt.Fprintln(os.Stderr, "error: plan file argument required\n\nUsage: tfx analyze <plan.json>")
		os.Exit(1)
	}

	format := strings.ToLower(*formatFlag)
	if format != "text" && format != "json" {
		fmt.Fprintf(os.Stderr, "error: --format must be 'text' or 'json', got %q\n", *formatFlag)
		os.Exit(1)
	}

	// 1. Parse
	changes, err := parser.ParseFile(planFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse plan file: %v\n", err)
		os.Exit(1)
	}

	if len(changes) == 0 {
		// Nothing to analyze — plan is a no-op.
		emptyReport := report.Build(nil, nil)
		render(emptyReport, format)
		return
	}

	// 2. Normalize
	norm := &awsnorm.Normalizer{}
	var resources []normalizer.NormalizedResource
	for _, rc := range changes {
		nr, err := norm.Normalize(rc, *regionFlag)
		if err != nil {
			// Non-fatal: unknown provider resource — skip with a notice.
			fmt.Fprintf(os.Stderr, "warning: could not normalize %s (%s): %v\n", rc.Address, rc.Type, err)
			continue
		}
		resources = append(resources, nr)
	}

	// 3. Price
	pricer := &awspricing.Pricer{}
	var estimates []pricing.Estimate
	for _, nr := range resources {
		estimates = append(estimates, pricer.Estimate(nr))
	}

	// 4. Rules
	findings := awsrules.Run(
		rules.EvaluateContext{Resources: resources, Estimates: estimates},
		awsrules.AllRules(),
	)

	// 5. Build & render report
	r := report.Build(estimates, findings)
	render(r, format)

	// Exit non-zero if there are critical findings so CI pipelines can gate on it.
	if r.Summary.CountBySeverity[rules.SeverityCritical] > 0 {
		os.Exit(2)
	}
}

func render(r report.Report, format string) {
	var formatter report.Formatter
	switch format {
	case "json":
		formatter = &report.JSONFormatter{}
	default:
		formatter = &report.TextFormatter{}
	}

	out, err := formatter.Format(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to format report: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(string(out))
}
