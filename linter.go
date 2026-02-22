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

func lowercaseFirstRune(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func collectSensitiveMatches(s string, patterns []*regexp.Regexp) ([]string, int) {
	var matches []string
	firstOff := -1

	for _, re := range patterns {
		loc := re.FindStringIndex(s)
		if loc != nil {
			if firstOff == -1 || loc[0] < firstOff {
				firstOff = loc[0]
			}
			name := strings.TrimPrefix(re.String(), "(?i)")
			matches = append(matches, name)
		}
	}
	return matches, firstOff
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

func (p *Plugin) checkValue(pass *analysis.Pass, expr ast.Expr, isMessage bool) {
	if p.settings == nil {
		return
	}

	if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		p.checkValue(pass, bin.X, isMessage)
		p.checkValue(pass, bin.Y, isMessage)
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

	if isMessage && p.settings.CheckLowercase {
		runes := []rune(orig)
		if len(runes) > 0 && unicode.IsUpper(runes[0]) {
			pass.Report(analysis.Diagnostic{
				Pos:     lit.Pos() + 1,
				Message: fmt.Sprintf("message %q starts with uppercase", orig),
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "lowercase first letter",
					TextEdits: []analysis.TextEdit{
						*buildLiteralFix(lit, lowercaseFirstRune(orig)),
					},
				}},
			})
		}
	}

	if p.settings.CheckEnglish {
		nonEnglish, firstOff := collectNonEnglishLetters(orig)
		if nonEnglish != "" {
			pass.Report(analysis.Diagnostic{
				Pos:     lit.Pos() + 1 + token.Pos(firstOff),
				Message: fmt.Sprintf("forbidden non-english characters found: {%s}", nonEnglish),
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "remove non-english letters",
					TextEdits: []analysis.TextEdit{
						*buildLiteralFix(lit, removeNonEnglishLetters(orig)),
					},
				}},
			})
		}
	}

	if p.settings.CheckSymbols {
		badSymbols, firstOff := collectBadSymbols(orig)
		if badSymbols != "" {
			pass.Report(analysis.Diagnostic{
				Pos:     lit.Pos() + 1 + token.Pos(firstOff),
				Message: fmt.Sprintf("message contains emojis or special symbols: {%s}", badSymbols),
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "remove special symbols",
					TextEdits: []analysis.TextEdit{
						*buildLiteralFix(lit, removeBadSymbols(orig)),
					},
				}},
			})
		}
	}

	if p.settings.CheckSensitive {
		matches, firstOff := collectSensitiveMatches(orig, p.compiledPatterns)
		if len(matches) > 0 {
			diag := analysis.Diagnostic{
				Pos:      lit.Pos() + 1 + token.Pos(firstOff),
				Category: "security",
				Message:  fmt.Sprintf("potential sensitive data exposure (found: %s)", strings.Join(matches, ", ")),
			}

			if edit := buildLiteralFix(lit, redactSensitive(orig, p.compiledPatterns)); edit != nil {
				diag.SuggestedFixes = []analysis.SuggestedFix{{
					Message:   "redact sensitive data",
					TextEdits: []analysis.TextEdit{*edit},
				}}
			}
			pass.Report(diag)
		}
	}
}

func collectNonEnglishLetters(s string) (string, int) {
	var bad []rune
	firstByteOff := -1
	for i, r := range s {
		if unicode.IsLetter(r) && r > 127 {
			if firstByteOff == -1 {
				firstByteOff = i
			}
			bad = append(bad, r)
		}
	}
	if firstByteOff == -1 {
		return "", 0
	}
	return string(bad), firstByteOff
}

func removeNonEnglishLetters(s string) string {
	var filtered []rune
	for _, r := range s {
		if unicode.IsLetter(r) {
			if r <= 127 {
				filtered = append(filtered, r)
			}
			continue
		}
		filtered = append(filtered, r)
	}
	return string(filtered)
}

func collectBadSymbols(s string) (string, int) {
	var bad []rune
	firstByteOff := -1
	for i, r := range s {
		if unicode.IsPunct(r) || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r)) {
			if firstByteOff == -1 {
				firstByteOff = i
			}
			bad = append(bad, r)
		}
	}
	if firstByteOff == -1 {
		return "", 0
	}
	return string(bad), firstByteOff
}

func removeBadSymbols(s string) string {
	var filtered []rune
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			filtered = append(filtered, r)
		}
	}
	return string(filtered)
}
