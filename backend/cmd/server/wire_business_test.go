package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGeneratedBusinessWiring guards the production graph: handler tests alone
// cannot catch a merge replacing enriched providers with bare constructors.
func TestGeneratedBusinessWiring(t *testing.T) {
	source, err := parser.ParseFile(token.NewFileSet(), "wire_gen.go", nil, 0)
	require.NoError(t, err)
	calls := make(map[string]bool)
	fields := make(map[string]map[string]bool)
	ast.Inspect(source, func(node ast.Node) bool {
		switch expr := node.(type) {
		case *ast.CallExpr:
			calls[qualifiedBusinessName(expr.Fun)] = true
		case *ast.CompositeLit:
			fields[qualifiedBusinessName(expr.Type)] = populatedBusinessFields(expr)
		}
		return true
	})
	for _, name := range []string{
		"handler.ProvideUserHandler", "handler.ProvideAdminUserHandler",
		"handler.ProvideAdminSettingHandler", "handler.ProvideDashboardHandler",
		"handler.ProvideAdminHandlers", "admin.NewWelfareHandler",
		"service.NewLotteryService", "service.NewQQBindingService",
		"service.NewBusinessAnalyticsService", "service.ProvideWelfareService",
	} {
		require.True(t, calls[name], "production graph must call %s", name)
	}
	for name, required := range map[string][]string{
		"handler.UserHandlerDependencies":         {"LotteryService"},
		"handler.AdminUserHandlerDependencies":    {"LotteryService", "QQBindingService"},
		"handler.AdminSettingHandlerDependencies": {"LotteryService"},
		"handler.DashboardHandlerDependencies":    {"BusinessService"},
		"handler.AdminHandlersDependencies":       {"WelfareHandler"},
	} {
		for _, field := range required {
			require.True(t, fields[name][field], "%s.%s must be injected", name, field)
		}
	}
}

func qualifiedBusinessName(expr ast.Expr) string {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return pkg.Name + "." + selector.Sel.Name
}

func populatedBusinessFields(literal *ast.CompositeLit) map[string]bool {
	fields := make(map[string]bool)
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, keyOK := pair.Key.(*ast.Ident)
		value, valueOK := pair.Value.(*ast.Ident)
		if keyOK && valueOK && value.Name != "nil" {
			fields[key.Name] = true
		}
	}
	return fields
}
