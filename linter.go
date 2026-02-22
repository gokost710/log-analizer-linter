package linter

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

var logMethods = map[string]bool{
	"Debug": true, "Info": true, "Warn": true, "Warning": true, "Error": true,
	"DPanic": true, "Panic": true, "Fatal": true,
	"DebugContext": true, "InfoContext": true, "WarnContext": true, "ErrorContext": true,
	"Debugf": true, "Infof": true, "Warnf": true, "Errorf": true, "Panicf": true, "Fatalf": true,
	"Debugw": true, "Infow": true, "Warnw": true, "Errorw": true,
	"Log": true, "LogAttrs": true,
}

type MySettings struct {
	CheckLowercase    bool     `json:"check-lowercase"`
	CheckEnglish      bool     `json:"check-english"`
	CheckSymbols      bool     `json:"check-symbols"`
	CheckSensitive    bool     `json:"check-sensitive"`
	SensitivePatterns []string `json:"sensitive-patterns"`
}

type Plugin struct {
	settings         *MySettings
	compiledPatterns []*regexp.Regexp
}

func init() {
	register.Plugin("gologanalizer", New)
}

func New(settings any) (register.LinterPlugin, error) {
	s := MySettings{
		CheckLowercase: true,
		CheckEnglish:   true,
		CheckSymbols:   true,
		CheckSensitive: true,
	}

	if settings != nil {
		decoded, err := register.DecodeSettings[MySettings](settings)
		if err != nil {
			return nil, err
		}
		s = mergeSettings(s, decoded)
	}

	p := &Plugin{settings: &s}

	if s.CheckSensitive {
		for _, pattern := range s.SensitivePatterns {
			re, err := regexp.Compile("(?i)" + pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
			}
			p.compiledPatterns = append(p.compiledPatterns, re)
		}
	}

	return p, nil
}

func mergeSettings(def, user MySettings) MySettings {
	def.CheckLowercase = user.CheckLowercase
	def.CheckEnglish = user.CheckEnglish
	def.CheckSymbols = user.CheckSymbols
	def.CheckSensitive = user.CheckSensitive

	if len(user.SensitivePatterns) > 0 {
		def.SensitivePatterns = user.SensitivePatterns
	}

	return def
}

func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{{
		Name: "gologanalizer",
		Doc:  "Checks logging style and sensitive data exposure",
		Run:  p.run,
	}}, nil
}

func (p *Plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

func (p *Plugin) run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if p.isLogCall(pass, sel) {
				p.analyzeLogCall(pass, sel.Sel.Name, call.Args)
			}

			return true
		})
	}
	return nil, nil
}

func (p *Plugin) isLogCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.Type != nil {
		tStr := tv.Type.String()
		if strings.Contains(tStr, "log/slog") || strings.Contains(tStr, "go.uber.org/zap") {
			return true
		}
	}

	if ident, ok := sel.X.(*ast.Ident); ok {
		if obj, ok := pass.TypesInfo.Uses[ident]; ok {
			if pkg, ok := obj.(*types.PkgName); ok {
				path := pkg.Imported().Path()
				if path == "log/slog" || strings.Contains(path, "go.uber.org/zap") {
					return true
				}
			}
		}
	}

	if xCall, ok := sel.X.(*ast.CallExpr); ok {
		if xSel, ok := xCall.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := xSel.X.(*ast.Ident); ok && ident.Name == "zap" {
				return true
			}
		}
	}

	return false
}

func (p *Plugin) analyzeLogCall(pass *analysis.Pass, methodName string, args []ast.Expr) {
	if !logMethods[methodName] || len(args) == 0 {
		return
	}

	msgIdx := 0
	if strings.HasSuffix(methodName, "Context") {
		msgIdx = 1
	}

	if len(args) <= msgIdx {
		return
	}

	p.checkValue(pass, args[msgIdx], true)

	for i, arg := range args {
		if i == msgIdx {
			continue
		}
		p.checkValue(pass, arg, false)
	}
}

func (p *Plugin) checkValue(pass *analysis.Pass, expr ast.Expr, isMessage bool) {
	if p.settings == nil {
		return
	}

	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}

	orig, err := strconv.Unquote(lit.Value)
	if err != nil || orig == "" {
		return
	}

	var diags []analysis.Diagnostic
	fixed := orig

	if isMessage && p.settings.CheckLowercase {
		runes := []rune(orig)
		if len(runes) > 0 && unicode.IsUpper(runes[0]) {
			diags = append(diags, analysis.Diagnostic{
				Pos:     lit.Pos() + 1,
				End:     lit.Pos() + 1 + token.Pos(len(string(runes[0]))),
				Message: fmt.Sprintf("gologanalizer: message %q starts with uppercase", orig),
			})
			fixed = lowercaseFirstRune(fixed)
		}
	}
	if p.settings.CheckEnglish {
		nonEnglish, firstOff := collectNonEnglishLetters(orig)
		if nonEnglish != "" {
			diags = append(diags, analysis.Diagnostic{
				Pos:     lit.Pos() + 1 + token.Pos(firstOff),
				Message: fmt.Sprintf("gologanalizer: forbidden non-english characters found: %s", nonEnglish),
			})
			fixed = removeNonEnglishLetters(fixed)
		}
	}
	if p.settings.CheckSymbols {
		badSymbols := collectBadSymbols(orig)
		if badSymbols != "" {
			diags = append(diags, analysis.Diagnostic{
				Pos:     lit.Pos() + 1,
				Message: fmt.Sprintf("gologanalizer: message contains emojis or special symbols: %s", badSymbols),
			})
			fixed = removeBadSymbols(fixed)
		}
	}
	if p.settings.CheckSensitive {
		matches := collectSensitiveMatches(orig, p.compiledPatterns)
		if len(matches) > 0 {
			diags = append(diags, analysis.Diagnostic{
				Pos:      lit.Pos() + 1,
				Category: "security",
				Message:  fmt.Sprintf("gologanalizer: POTENTIAL SENSITIVE DATA EXPOSURE (matched: %s)", strings.Join(matches, ", ")),
			})
			fixed = redactSensitive(fixed, p.compiledPatterns)
		}
	}

	if len(diags) == 0 {
		return
	}

	if edit := buildLiteralFix(lit, fixed); edit != nil {
		diags[0].SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "apply gologanalizer automatic fix",
			TextEdits: []analysis.TextEdit{*edit},
		}}
	}

	for _, d := range diags {
		pass.Report(d)
	}
}

func lowercaseFirstRune(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func collectNonEnglishLetters(s string) (string, int) {
	var nonEnglish []rune
	firstByteOff := -1

	for i, r := range s {
		if r > 127 && !unicode.In(r, unicode.Latin) {
			if firstByteOff == -1 {
				firstByteOff = i
			}
			nonEnglish = append(nonEnglish, r)
		}
	}

	if firstByteOff == -1 {
		return "", 0
	}

	return string(nonEnglish), firstByteOff
}

func removeNonEnglishLetters(s string) string {
	var filtered []rune
	for _, r := range []rune(s) {
		if r > 127 && !unicode.In(r, unicode.Latin) {
			continue
		}
		filtered = append(filtered, r)
	}
	return string(filtered)
}

func collectBadSymbols(s string) string {
	var bad []rune
	for _, r := range []rune(s) {
		if r > 127 && (!unicode.IsLetter(r) && !unicode.IsDigit(r) || unicode.In(r, unicode.Greek)) {
			bad = append(bad, r)
		}
	}
	return string(bad)
}

func removeBadSymbols(s string) string {
	var filtered []rune
	for _, r := range []rune(s) {
		if r > 127 && (!unicode.IsLetter(r) && !unicode.IsDigit(r) || unicode.In(r, unicode.Greek)) {
			continue
		}
		filtered = append(filtered, r)
	}
	return string(filtered)
}

func collectSensitiveMatches(s string, patterns []*regexp.Regexp) []string {
	var matches []string
	for _, re := range patterns {
		if re.MatchString(s) {
			matches = append(matches, re.String())
		}
	}
	return matches
}

func redactSensitive(s string, patterns []*regexp.Regexp) string {
	out := s
	for _, re := range patterns {
		if re.MatchString(out) {
			out = re.ReplaceAllString(out, "secret")
		}
	}
	return out
}

func buildLiteralFix(lit *ast.BasicLit, newVal string) *analysis.TextEdit {
	if lit == nil || len(lit.Value) < 2 {
		return nil
	}

	oldVal, err := strconv.Unquote(lit.Value)
	if err == nil && newVal == oldVal {
		return nil
	}

	newLiteral := strconv.Quote(newVal)

	return &analysis.TextEdit{
		Pos:     lit.Pos(),
		End:     lit.End(),
		NewText: []byte(newLiteral),
	}
}
