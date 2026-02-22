package linter

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func TestVerifyLowercase(t *testing.T) {
	p := &Plugin{settings: &MySettings{CheckLowercase: true}}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Valid lowercase", "hello world", false},
		{"Invalid uppercase", "Hello world", true},
		{"Empty string", "", false},
		{"Numbers first", "123 hello", false},
		{"Special chars first", "!hello", false},
		{"Cyrillic uppercase", "Привет", true},
		{"Cyrillic lowercase", "привет", false},
		{"Single space", " ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lit := &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", tt.input)}
			pass := &analysis.Pass{
				Report: func(d analysis.Diagnostic) {
					if !tt.wantErr {
						t.Errorf("Unexpected error reported: %s", d.Message)
					}
				},
			}
			p.checkValue(pass, lit, true)
		})
	}
}

func TestVerifySensitive(t *testing.T) {
	p := &Plugin{
		compiledPatterns: []*regexp.Regexp{
			regexp.MustCompile("(?i)password"),
			regexp.MustCompile("(?i)token"),
		},
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Plain text", "normal message", false},
		{"Contains password", "my password is 123", true},
		{"Uppercase password", "PASSWORD_HERE", true},
		{"Token check", "found a token!", true},
		{"Substring match", "mytokenisgreat", true},
		{"Safe word", "passing through", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reported := false
			lit := &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", tt.input)}
			pass := &analysis.Pass{
				Report: func(d analysis.Diagnostic) {
					reported = true
				},
			}
			p.settings = &MySettings{CheckSensitive: true}
			p.checkValue(pass, lit, true)
			if reported != tt.wantErr {
				t.Errorf("input: %s, wantErr: %v, got: %v", tt.input, tt.wantErr, reported)
			}
		})
	}
}

func TestVerifySymbolsAndEnglish(t *testing.T) {
	p := &Plugin{settings: &MySettings{CheckEnglish: true, CheckSymbols: true}}

	tests := []struct {
		name        string
		input       string
		wantEnglish bool
		wantSymbols bool
	}{
		{"Pure English", "hello", false, false},
		{"Mixed Russian", "hello привет", true, false},
		{"Emoji", "good luck 🍀", true, true},
		{"Mathematical", "Σ sum is zero", true, true}, // Сигма - не латиница
		{"Chinese", "你好", true, false},
		{"Control chars", "hello\nworld\t", false, false}, // \n и \t обычно разрешены
		{"Zero-width space", "hello\u200Bworld", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engErr, symErr := false, false
			lit := &ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", tt.input)}
			pass := &analysis.Pass{
				Report: func(d analysis.Diagnostic) {
					if strings.HasPrefix(d.Message, "gologanalizer: forbidden non-english characters found:") {
						engErr = true
					}
					if strings.HasPrefix(d.Message, "gologanalizer: message contains emojis or special symbols:") {
						symErr = true
					}
				},
			}
			p.checkValue(pass, lit, true)

			if engErr != tt.wantEnglish {
				t.Errorf("%s: English error mismatch. Got %v", tt.name, engErr)
			}
			if symErr != tt.wantSymbols {
				t.Errorf("%s: Symbols error mismatch. Got %v", tt.name, symErr)
			}
		})
	}
}

func TestAnalyzeLogCall(t *testing.T) {
	p := &Plugin{settings: &MySettings{CheckLowercase: true}}

	argMsg := &ast.BasicLit{Kind: token.STRING, Value: `"Bad"`}
	argCtx := &ast.Ident{Name: "ctx"}

	t.Run("Slog Info - message at index 0", func(t *testing.T) {
		reported := false
		pass := &analysis.Pass{
			Report: func(d analysis.Diagnostic) { reported = true },
		}
		p.analyzeLogCall(pass, "Info", []ast.Expr{argMsg})
		if !reported {
			t.Error("Expected report for uppercase at index 0")
		}
	})

	t.Run("Slog InfoContext - message at index 1", func(t *testing.T) {
		reported := false
		pass := &analysis.Pass{
			Report: func(d analysis.Diagnostic) { reported = true },
		}
		p.analyzeLogCall(pass, "InfoContext", []ast.Expr{argCtx, argMsg})
		if !reported {
			t.Error("Expected report for uppercase at index 1")
		}
	})
}

func TestNew_SensitivePatternsCompiled(t *testing.T) {
	pluginAny, err := New(MySettings{
		CheckSensitive:    true,
		SensitivePatterns: []string{"password", "token"},
	})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	p, ok := pluginAny.(*Plugin)
	if !ok {
		t.Fatalf("New() returned type %T, want *Plugin", pluginAny)
	}

	if got, want := len(p.compiledPatterns), 2; got != want {
		t.Fatalf("compiledPatterns len = %d, want %d", got, want)
	}
}

func TestNew_InvalidPattern(t *testing.T) {
	_, err := New(MySettings{
		CheckSensitive:    true,
		SensitivePatterns: []string{"["},
	})
	if err == nil {
		t.Fatal("expected error for invalid regexp pattern, got nil")
	}
}

func TestCheckValue_RespectsFlags(t *testing.T) {
	lit := &ast.BasicLit{Kind: token.STRING, Value: `"Bad password 🍀"`}

	tests := []struct {
		name       string
		settings   MySettings
		isMessage  bool
		wantReport bool
	}{
		{
			name:       "AllDisabled",
			settings:   MySettings{},
			isMessage:  true,
			wantReport: false,
		},
		{
			name: "OnlyLowercase_Message",
			settings: MySettings{
				CheckLowercase: true,
			},
			isMessage:  true,
			wantReport: true,
		},
		{
			name: "OnlyLowercase_NonMessage",
			settings: MySettings{
				CheckLowercase: true,
			},
			isMessage:  false,
			wantReport: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plugin{settings: &tt.settings}
			reported := false
			pass := &analysis.Pass{
				Report: func(d analysis.Diagnostic) {
					reported = true
				},
			}

			p.checkValue(pass, lit, tt.isMessage)

			if reported != tt.wantReport {
				t.Fatalf("reported = %v, want %v", reported, tt.wantReport)
			}
		})
	}
}

func TestIsLogCall_SlogPackage(t *testing.T) {
	p := &Plugin{}

	pkgIdent := &ast.Ident{Name: "slog"}
	sel := &ast.SelectorExpr{
		X:   pkgIdent,
		Sel: ast.NewIdent("Info"),
	}

	info := &types.Info{
		Uses: map[*ast.Ident]types.Object{
			pkgIdent: types.NewPkgName(token.NoPos, nil, "slog", types.NewPackage("log/slog", "slog")),
		},
	}

	pass := &analysis.Pass{
		TypesInfo: info,
	}

	if !p.isLogCall(pass, sel) {
		t.Fatal("expected isLogCall to return true for log/slog.Info")
	}
}

func TestIsLogCall_ZapLogger(t *testing.T) {
	p := &Plugin{}

	zapIdent := &ast.Ident{Name: "zap"}
	call := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   zapIdent,
			Sel: ast.NewIdent("L"),
		},
	}
	sel := &ast.SelectorExpr{
		X:   call,
		Sel: ast.NewIdent("Info"),
	}

	pass := &analysis.Pass{
		TypesInfo: &types.Info{},
	}

	if !p.isLogCall(pass, sel) {
		t.Fatal("expected isLogCall to return true for zap.L().Info")
	}
}

func TestIsLogCall_NonLogger(t *testing.T) {
	p := &Plugin{}

	ident := &ast.Ident{Name: "fmt"}
	sel := &ast.SelectorExpr{
		X:   ident,
		Sel: ast.NewIdent("Println"),
	}

	pass := &analysis.Pass{
		TypesInfo: &types.Info{},
	}

	if p.isLogCall(pass, sel) {
		t.Fatal("expected isLogCall to return false for non-logger package")
	}
}

func TestRun_ReportsDiagnosticsForLogCalls(t *testing.T) {
	p := &Plugin{
		settings: &MySettings{
			CheckLowercase: true,
			CheckEnglish:   false,
			CheckSymbols:   false,
			CheckSensitive: false,
		},
	}

	fset := token.NewFileSet()
	pkgIdent := &ast.Ident{Name: "slog"}
	call := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   pkgIdent,
			Sel: ast.NewIdent("Info"),
		},
		Args: []ast.Expr{
			&ast.BasicLit{Kind: token.STRING, Value: `"Bad message"`},
		},
	}

	file := &ast.File{
		Name: ast.NewIdent("main"),
		Decls: []ast.Decl{
			&ast.FuncDecl{
				Name: ast.NewIdent("f"),
				Type: &ast.FuncType{},
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						&ast.ExprStmt{X: call},
					},
				},
			},
		},
	}

	info := &types.Info{
		Uses: map[*ast.Ident]types.Object{
			pkgIdent: types.NewPkgName(token.NoPos, nil, "slog", types.NewPackage("log/slog", "slog")),
		},
	}

	var diags []analysis.Diagnostic
	pass := &analysis.Pass{
		Fset:      fset,
		Files:     []*ast.File{file},
		TypesInfo: info,
		Report: func(d analysis.Diagnostic) {
			diags = append(diags, d)
		},
	}

	if _, err := p.run(pass); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	if len(diags) == 0 {
		t.Fatal("expected diagnostics for uppercase log message, got none")
	}
}

func TestBuildAnalyzersAndGetLoadMode(t *testing.T) {
	p := &Plugin{}

	analyzers, err := p.BuildAnalyzers()
	if err != nil {
		t.Fatalf("BuildAnalyzers() error = %v", err)
	}

	if len(analyzers) != 1 {
		t.Fatalf("expected 1 analyzer, got %d", len(analyzers))
	}

	if analyzers[0].Name != "gologanalizer" {
		t.Fatalf("analyzer name = %q, want %q", analyzers[0].Name, "gologanalizer")
	}

	if got := p.GetLoadMode(); got != register.LoadModeTypesInfo {
		t.Fatalf("GetLoadMode() = %q, want %q", got, register.LoadModeTypesInfo)
	}
}
