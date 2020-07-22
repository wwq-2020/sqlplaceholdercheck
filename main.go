package main

import (
	"errors"
	"fmt"
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

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if ce.Fun == nil {
				return true
			}

			se, ok := ce.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if se.Sel == nil {
				return true
			}
			needArg := 1
			handler := func(stmt sqlAST.StmtNode, args []ast.Expr, hasEllipsis bool) (int, error) {
				return 0, nil
			}
			switch se.Sel.Name {
			case "Query":
				handler = handleQuery
			case "QueryContext":
				handler = handleQueryContext
				needArg++
			case "Exec":
				handler = handleExec
			case "ExecContext":
				handler = handleExecContext
				needArg++
			case "QueryRow":
				handler = handleQueryRow
			case "QueryRowContext":
				handler = handleQueryRowContext
				needArg++
			case "Scan":
				ce2, ok := se.X.(*ast.CallExpr)
				if !ok {
					if err := handleScanForQuery(ce, se); err != nil {
						pass.Reportf(n.Pos(), err.Error())
					}
					return true

				}
				if ce2.Fun == nil {
					return true
				}
				se2, ok := ce2.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if err := handleScanForQueryRow(ce, ce2, se, se2); err != nil {
					pass.Reportf(n.Pos(), err.Error())
				}
				return true
			default:
				return true
			}

			if len(ce.Args) < needArg {
				pass.Reportf(n.Pos(), "args mismatch")
				return true
			}

			p := parser.New()
			astSQL, ok := ce.Args[needArg-1].(*ast.BasicLit)
			if !ok {
				return true
			}

			sql := strings.TrimPrefix(astSQL.Value, "\"")
			sql = strings.TrimSuffix(sql, "\"")
			sns, _, err := p.Parse(sql, "utf8mb4", "utf8mb4")
			if err != nil {
				return true
			}
			sn := sns[0]
			if _, err := handler(sn, ce.Args[needArg:], ce.Ellipsis.IsValid()); err != nil {
				pass.Reportf(n.Pos(), err.Error())
			}
			return true
		})
	}
	return nil, nil
}

func handleScanForQuery(ce *ast.CallExpr, se *ast.SelectorExpr) error {
	if se.X == nil {
		return nil
	}
	i, ok := se.X.(*ast.Ident)
	if !ok {
		return nil
	}
	if i.Obj == nil || i.Obj.Decl == nil {
		return nil
	}
	a, ok := i.Obj.Decl.(*ast.AssignStmt)
	if !ok {
		return nil
	}
	if len(a.Rhs) == 0 {
		return nil
	}
	ce2, ok := a.Rhs[0].(*ast.CallExpr)
	if !ok {
		return nil
	}
	if ce2.Fun == nil {
		return nil
	}
	se2, ok := ce2.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	if se2.Sel == nil {
		return nil
	}

	needArg := 1
	handler := func(stmt sqlAST.StmtNode, args []ast.Expr, hasEllipsis bool) (int, error) {
		return 0, nil
	}
	switch se2.Sel.Name {
	case "Query":
		handler = handleQuery
	case "QueryContext":
		handler = handleQueryContext
		needArg++
	default:
		return nil
	}
	if len(ce2.Args) < needArg {
		return nil
	}

	p := parser.New()
	astSQL, ok := ce2.Args[needArg-1].(*ast.BasicLit)
	if !ok {
		return nil
	}

	sql := strings.TrimPrefix(astSQL.Value, "\"")
	sql = strings.TrimSuffix(sql, "\"")
	sns, _, err := p.Parse(sql, "utf8mb4", "utf8mb4")
	if err != nil {
		return nil
	}
	sn := sns[0]
	placeHolderNum, err := handler(sn, ce2.Args[needArg:], ce2.Ellipsis.IsValid())
	if err != nil {
		return nil
	}
	if placeHolderNum != len(ce.Args) {
		return errors.New("scan arg mismatch")
	}
	return nil

}

func handleScanForQueryRow(ce, ce2 *ast.CallExpr, se, se2 *ast.SelectorExpr) error {
	if se2.Sel == nil {
		return nil
	}
	needArg := 1
	handler := func(stmt sqlAST.StmtNode, args []ast.Expr, hasEllipsis bool) (int, error) {
		return 0, nil
	}
	switch se2.Sel.Name {
	case "QueryRow":
		handler = handleQueryRow
	default:
		return nil
	}
	if len(ce2.Args) < needArg {
		return nil
	}

	p := parser.New()
	astSQL, ok := ce2.Args[needArg-1].(*ast.BasicLit)
	if !ok {
		return nil
	}

	sql := strings.TrimPrefix(astSQL.Value, "\"")
	sql = strings.TrimSuffix(sql, "\"")
	sns, _, err := p.Parse(sql, "utf8mb4", "utf8mb4")
	if err != nil {
		return nil
	}
	sn := sns[0]
	placeHolderNum, err := handler(sn, ce2.Args[needArg:], ce2.Ellipsis.IsValid())
	if err != nil {
		return nil
	}
	if placeHolderNum != len(ce.Args) {
		return errors.New("scan arg mismatch")
	}
	return nil
}

func handleQuery(stmt sqlAST.StmtNode, args []ast.Expr, hasEllipsis bool) (int, error) {
	if stmt == nil {
		return 0, nil
	}
	ss, ok := stmt.(*sqlAST.SelectStmt)
	if !ok {
		return 0, errors.New("not select in do Query")
	}
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if ss.Limit != nil {
		if ss.Limit.Offset != nil {
			placeHolderNum++
		}
		if ss.Limit.Count != nil {
			placeHolderNum++
		}
	}
	if placeHolderNum != 0 && len(args) != 0 && hasEllipsis {
		return placeHolderNum, nil
	}
	if placeHolderNum != len(args) {
		return 0, errors.New("query arg count mismatch")
	}
	return placeHolderNum, nil
}

func handleQueryContext(stmt sqlAST.StmtNode, args []ast.Expr, hasEllipsis bool) (int, error) {
	if stmt == nil {
		return 0, nil
	}
	ss, ok := stmt.(*sqlAST.SelectStmt)
	if !ok {
		return 0, errors.New("not select in do Query")
	}
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if ss.Limit != nil {
		if ss.Limit.Offset != nil {
			placeHolderNum++
		}
		if ss.Limit.Count != nil {
			placeHolderNum++
		}
	}
	if placeHolderNum != 0 && len(args) != 0 && hasEllipsis {
		return placeHolderNum, nil
	}
	if placeHolderNum != len(args) {
		return 0, errors.New("query arg count mismatch")
	}
	return placeHolderNum, nil
}

func handleExecContext(stmt sqlAST.StmtNode, args []ast.Expr, hasEllipsis bool) (int, error) {
	if stmt == nil {
		return 0, nil
	}
	switch t := stmt.(type) {
	case *sqlAST.InsertStmt:
		return handleInsert(t, args, hasEllipsis)
	case *sqlAST.DeleteStmt:
		return handleDelete(t, args, hasEllipsis)
	case *sqlAST.UpdateStmt:
		return handleUpdate(t, args, hasEllipsis)
	default:
		panic(fmt.Sprintf("unexpected stmt:%s", stmt.Text()))
	}

}

func handleInsert(ss *sqlAST.InsertStmt, args []ast.Expr, hasEllipsis bool) (int, error) {
	placeHolderNum := 0
	for _, each := range ss.Lists {
		placeHolderNum += len(each)
	}
	if placeHolderNum != 0 && len(args) != 0 && hasEllipsis {
		return placeHolderNum, nil
	}
	if placeHolderNum != len(args) {
		return 0, errors.New("argcnt mismatch")
	}
	return placeHolderNum, nil
}

func handleDelete(ss *sqlAST.DeleteStmt, args []ast.Expr, hasEllipsis bool) (int, error) {
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}

	if ss.Limit != nil {
		if ss.Limit.Offset != nil {
			placeHolderNum++
		}
		if ss.Limit.Count != nil {
			placeHolderNum++
		}
	}
	if placeHolderNum != 0 && len(args) != 0 && hasEllipsis {
		return placeHolderNum, nil
	}
	if placeHolderNum != len(args) {
		return 0, errors.New("delete arg count mismatch")
	}
	return placeHolderNum, nil
}

func handleUpdate(ss *sqlAST.UpdateStmt, args []ast.Expr, hasEllipsis bool) (int, error) {
	placeHolderNum := 0
	for _, each := range ss.List {
		_, ok := each.Expr.(*driver.ParamMarkerExpr)
		if ok {
			placeHolderNum++
		}
	}

	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if ss.Limit != nil {
		if ss.Limit.Offset != nil {
			placeHolderNum++
		}
		if ss.Limit.Count != nil {
			placeHolderNum++
		}
	}
	if placeHolderNum != 0 && len(args) != 0 && hasEllipsis {
		return placeHolderNum, nil
	}
	if placeHolderNum != len(args) {
		return 0, errors.New("update arg count mismatch")
	}
	return placeHolderNum, nil
}

func handleQueryRow(stmt sqlAST.StmtNode, args []ast.Expr, hasEllipsis bool) (int, error) {
	if stmt == nil {
		return 0, nil
	}
	ss, ok := stmt.(*sqlAST.SelectStmt)
	if !ok {
		return 0, errors.New("not select in do Query")
	}
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if ss.Limit != nil {
		if ss.Limit.Offset != nil {
			placeHolderNum++
		}
		if ss.Limit.Count != nil {
			placeHolderNum++
		}
	}
	if placeHolderNum != 0 && len(args) != 0 && hasEllipsis {
		return placeHolderNum, nil
	}
	if placeHolderNum != len(args) {
		return 0, errors.New("query arg count mismatch")
	}
	return placeHolderNum, nil
}

func handleQueryRowContext(stmt sqlAST.StmtNode, args []ast.Expr, hasEllipsis bool) (int, error) {
	if stmt == nil {
		return 0, nil
	}
	ss, ok := stmt.(*sqlAST.SelectStmt)
	if !ok {
		return 0, errors.New("not select in do Query")
	}
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if ss.Limit != nil {
		if ss.Limit.Offset != nil {
			placeHolderNum++
		}
		if ss.Limit.Count != nil {
			placeHolderNum++
		}
	}
	if placeHolderNum != 0 && len(args) != 0 && hasEllipsis {
		return placeHolderNum, nil
	}
	if placeHolderNum != len(args) {
		return 0, errors.New("query arg count mismatch")
	}
	return placeHolderNum, nil
}

func handleExec(stmt sqlAST.StmtNode, args []ast.Expr, hasEllipsis bool) (int, error) {
	if stmt == nil {
		return 0, nil
	}
	switch t := stmt.(type) {
	case *sqlAST.InsertStmt:
		return handleInsert(t, args, hasEllipsis)
	case *sqlAST.DeleteStmt:
		return handleDelete(t, args, hasEllipsis)
	case *sqlAST.UpdateStmt:
		return handleUpdate(t, args, hasEllipsis)
	default:
		panic(fmt.Sprintf("unexpected stmt:%s", stmt.Text()))
	}
}

func calcWherePlaceHolderNum(stmt *sqlAST.BinaryOperationExpr, cur int) int {
	if stmt.L != nil {
		switch t := stmt.L.(type) {
		case *sqlAST.BinaryOperationExpr:
			cur = calcWherePlaceHolderNum(t, cur)
		case *driver.ParamMarkerExpr:
			cur++
		default:
		}
	}
	if stmt.R != nil {
		switch t := stmt.R.(type) {
		case *sqlAST.BinaryOperationExpr:
			cur = calcWherePlaceHolderNum(t, cur)
		case *driver.ParamMarkerExpr:
			cur++
		default:
		}
	}
	return cur
}
