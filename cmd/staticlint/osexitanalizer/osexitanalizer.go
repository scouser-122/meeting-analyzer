package osexitanalizer

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// OsExitAnalyzer checks that os.Exit is not called directly in main function of main package
var OsExitAnalyzer = &analysis.Analyzer{
	Name: "noosexit",
	Doc:  "checks that os.Exit is not called directly in main function of main package",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	checkMainCallExpr := func(x *ast.CallExpr) {
		// check that this is function call
		selectorExpr, ok := x.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		if ident, ok := selectorExpr.X.(*ast.Ident); ok {
			if ident.Name == "os" && selectorExpr.Sel.Name == "Exit" {
				pass.Reportf(
					x.Pos(),
					"direct call to os.Exit in main function is forbidden",
				)
			}
		}
	}
	checkNotMainCallExpr := func(x *ast.CallExpr) {
		// check that this is function call
		selectorExpr, ok := x.Fun.(*ast.SelectorExpr)
		if ok {
			if ident, ok := selectorExpr.X.(*ast.Ident); ok {
				if ident.Name == "log" && selectorExpr.Sel.Name == "Fatal" {
					pass.Reportf(
						x.Pos(),
						"direct call to log.Fatal not in main function is forbidden",
					)
				}
			}
		} else {
			ident, ok := x.Fun.(*ast.Ident)
			if ok {
				if ident.Name == "panic" {
					pass.Reportf(
						x.Pos(),
						"direct call to panic not in main function is forbidden",
					)
				}
			}
		}
	}
	for _, file := range pass.Files {
		// go through all AST nodes
		pkgName := file.Name.Name
		ast.Inspect(file, func(node ast.Node) bool {
			fnDecl, ok := node.(*ast.FuncDecl)
			if ok {
				if pkgName == "main" && fnDecl.Name.Name == "main" {
					ast.Inspect(fnDecl.Body, func(innerNode ast.Node) bool {
						switch x := innerNode.(type) {
						case *ast.CallExpr:
							checkMainCallExpr(x)
						}
						return true
					})
				} else {
					ast.Inspect(fnDecl.Body, func(innerNode ast.Node) bool {
						switch x := innerNode.(type) {
						case *ast.CallExpr:
							checkNotMainCallExpr(x)
						}
						return true
					})
				}
			}
			return true
		})
	}
	return nil, nil
}
