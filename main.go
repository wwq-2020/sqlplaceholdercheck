package main

import (
	"go/ast"
	"strings"

	"github.com/pingcap/parser"
	sqlAST "github.com/pingcap/parser/ast"
	_ "github.com/pingcap/tidb/types/parser_driver"
	driver "github.com/pingcap/tidb/types/parser_driver"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	analyzer := &analysis.Analyzer{
		Name: "sqlplaceholdercheck",
		Doc:  "sqlplaceholdercheck",
		Run:  run,
	}
	singlechecker.Main(analyzer)
}

// &ast.CallExpr
// &ast.BlockStmt
func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			se, ok := ce.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if se.Sel.Name != "QueryContext" {
				return true
			}
			// 至少 ctx, sql
			if len(ce.Args) < 2 {
				return true
			}
			args := ce.Args[2:]

			p := parser.New()
			sql := strings.TrimPrefix(ce.Args[1].(*ast.BasicLit).Value, "\"")
			sql = strings.TrimSuffix(sql, "\"")
			sns, _, err := p.Parse(sql, "utf8mb4", "utf8mb4")
			if err != nil {
				return true
			}
			sn := sns[0]
			ss := sn.(*sqlAST.SelectStmt)
			placeHolderNum := calcPlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), 0)
			if placeHolderNum != len(args) {
				pass.Reportf(n.Pos(), "args mismatch")
				return true
			}
			return true
		})
	}
	return nil, nil
}

func calcPlaceHolderNum(stmt *sqlAST.BinaryOperationExpr, cur int) int {
	switch t := stmt.L.(type) {
	case *sqlAST.BinaryOperationExpr:
		cur = calcPlaceHolderNum(t, cur)
	case *driver.ParamMarkerExpr:
		cur++
	default:
	}
	switch t := stmt.R.(type) {
	case *sqlAST.BinaryOperationExpr:
		cur = calcPlaceHolderNum(t, cur)
	case *driver.ParamMarkerExpr:
		cur++
	default:
	}
	return cur
}
