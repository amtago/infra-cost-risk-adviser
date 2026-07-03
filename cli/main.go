// Package main is the entry point for the tfx CLI.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/amt/tf-cost-risk/cfn"
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
  tfx cfn <changeset.json> [--template template.json] [flags]
  tfx version

Flags:
  --format text|json   Output format (default: text)
  --region <region>    AWS region to use for pricing lookups (default: us-east-1)
  --help               Show this help

Examples:
  terraform show -json tfplan.binary > plan.json
  tfx analyze plan.json
  tfx analyze plan.json --format json --region eu-west-1

  aws cloudformation describe-change-set --change-set-name cs-1 --stack-name my-stack > cs.json
  tfx cfn cs.json
  tfx cfn cs.json --template template.json --format json
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "analyze":
		runAnalyze(os.Args[2:])
	case "cfn":
		runCFN(os.Args[2:])
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

	changes, err := parser.ParseFile(planFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse plan file: %v\n", err)
		os.Exit(1)
	}

	runPipeline(changes, *regionFlag, format)
}

func runCFN(args []string) {
	fs := flag.NewFlagSet("cfn", flag.ExitOnError)
	formatFlag := fs.String("format", "text", "Output format: text or json")
	regionFlag := fs.String("region", "us-east-1", "AWS region for pricing lookups")
	templateFlag := fs.String("template", "", "Path to the CloudFormation template JSON (optional)")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	var changeSetFile string
	var flagArgs []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && changeSetFile == "" {
			changeSetFile = a
		} else {
			flagArgs = append(flagArgs, a)
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(1)
	}

	if changeSetFile == "" {
		fmt.Fprintln(os.Stderr, "error: change set file argument required\n\nUsage: tfx cfn <changeset.json>")
		os.Exit(1)
	}

	format := strings.ToLower(*formatFlag)
	if format != "text" && format != "json" {
		fmt.Fprintf(os.Stderr, "error: --format must be 'text' or 'json', got %q\n", *formatFlag)
		os.Exit(1)
	}

	changes, err := cfn.ParseFile(changeSetFile, *templateFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to parse change set: %v\n", err)
		os.Exit(1)
	}

	runPipeline(changes, *regionFlag, format)
}

// runPipeline executes normalize → price → rules → report and renders output.
// Shared by runAnalyze and runCFN.
func runPipeline(changes []parser.ResourceChange, region, format string) {
	if len(changes) == 0 {
		render(report.Build(nil, nil), format)
		return
	}

	norm := &awsnorm.Normalizer{}
	var resources []normalizer.NormalizedResource
	for _, rc := range changes {
		nr, err := norm.Normalize(rc, region)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not normalize %s (%s): %v\n", rc.Address, rc.Type, err)
			continue
		}
		resources = append(resources, nr)
	}

	pricer := &awspricing.Pricer{}
	var estimates []pricing.Estimate
	for _, nr := range resources {
		estimates = append(estimates, pricer.Estimate(nr))
	}

	findings := awsrules.Run(
		rules.EvaluateContext{Resources: resources, Estimates: estimates},
		awsrules.AllRules(),
	)

	r := report.Build(estimates, findings)
	render(r, format)

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
