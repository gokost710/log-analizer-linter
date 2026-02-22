package linter

import (
	"fmt"
	"go/ast"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

type MySettings struct {
	CheckLowercase    *bool     `json:"check-lowercase"`
	CheckEnglish      *bool     `json:"check-english"`
	CheckSymbols      *bool     `json:"check-symbols"`
	CheckSensitive    *bool     `json:"check-sensitive"`
	SensitivePatterns *[]string `json:"sensitive-patterns"`
}

type Plugin struct {
	settings *MySettings
}

func init() {
	register.Plugin("gologanalizer", New)
}

func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[MySettings](settings)
	if err != nil {
		return nil, err
	}

	return &Plugin{settings: &s}, nil
}

func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{
		{
			Name: "gologanalizer",
			Doc:  "Check Go log analysis",
			Run:  p.run,
		},
	}, nil
}

func (p *Plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}

func (p *Plugin) run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				fmt.Println(call)
			}

			return true
		})
	}

	return nil, nil
}
