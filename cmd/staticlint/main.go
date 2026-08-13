package main

import (
	"strings"

	"github.com/gordonklaus/ineffassign/pkg/ineffassign"
	"github.com/scouser-122/meeting-analyzer/cmd/staticlint/osexitanalizer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"honnef.co/go/tools/staticcheck"
)

func main() {
	var mychecks []*analysis.Analyzer

	// os.Exit call in main checker
	mychecks = append(mychecks, osexitanalizer.OsExitAnalyzer)

	// some static analizers from staticcheck package
	staticChecks := map[string]bool{
		"S1000":  true, //
		"ST1003": true,
		"QF1003": true,
	}
	for _, v := range staticcheck.Analyzers {
		// all static analizers of SA class from staticcheck package
		if strings.HasPrefix(v.Analyzer.Name, "SA") {
			mychecks = append(mychecks, v.Analyzer)
		}
		// some static analizers from staticcheck package
		if staticChecks[v.Analyzer.Name] {
			mychecks = append(mychecks, v.Analyzer)
		}
	}

	// static analizers from analysis/passes package
	mychecks = append(mychecks, printf.Analyzer)
	mychecks = append(mychecks, shadow.Analyzer)
	mychecks = append(mychecks, structtag.Analyzer)
	mychecks = append(mychecks, nilness.Analyzer)

	// ineffectual assignments checker
	mychecks = append(mychecks, ineffassign.Analyzer)

	multichecker.Main(
		mychecks...,
	)
}
