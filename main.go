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

			se, ok := ce.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			needArg := 1
			handler := func(stmt sqlAST.StmtNode, argCnt int) error {
				return nil
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
			default:
				return true
			}

			if len(ce.Args) < needArg {
				pass.Reportf(n.Pos(), "args mismatch")
				return true
			}
			argCnt := len(ce.Args[needArg:])

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
			if err := handler(sn, argCnt); err != nil {
				pass.Reportf(n.Pos(), err.Error())
			}
			return true
		})
	}
	return nil, nil
}

func handleQuery(stmt sqlAST.StmtNode, argCnt int) error {
	ss, ok := stmt.(*sqlAST.SelectStmt)
	if ok {
		return errors.New("not select in do Query")
	}
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if placeHolderNum != argCnt {
		return errors.New("argcnt mismatch")
	}
	return nil
}

func handleQueryContext(stmt sqlAST.StmtNode, argCnt int) error {
	ss, ok := stmt.(*sqlAST.SelectStmt)
	if ok {
		return errors.New("not select in do Query")
	}
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if placeHolderNum != argCnt {
		return errors.New("argcnt mismatch")
	}
	return nil
}

func handleExecContext(stmt sqlAST.StmtNode, argCnt int) error {
	switch t := stmt.(type) {
	case *sqlAST.InsertStmt:
		return handleInsert(t, argCnt)
	case *sqlAST.DeleteStmt:
		return handleDelete(t, argCnt)
	case *sqlAST.UpdateStmt:
		return handleUpdate(t, argCnt)
	default:
		panic(fmt.Sprintf("unexpected stmt:%s", stmt.Text()))
	}

}

func handleInsert(ss *sqlAST.InsertStmt, argCnt int) error {
	placeHolderNum := 0
	for _, each := range ss.Lists {
		placeHolderNum += len(each)
	}
	if placeHolderNum != argCnt {
		return errors.New("argcnt mismatch")
	}
	return nil
}

func handleDelete(ss *sqlAST.DeleteStmt, argCnt int) error {
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if placeHolderNum != argCnt {
		return errors.New("argcnt mismatch")
	}
	return nil
}

func handleUpdate(ss *sqlAST.UpdateStmt, argCnt int) error {
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
	if placeHolderNum != argCnt {
		return errors.New("argcnt mismatch")
	}
	return nil

}

func handleQueryRow(stmt sqlAST.StmtNode, argCnt int) error {
	ss, ok := stmt.(*sqlAST.SelectStmt)
	if ok {
		return errors.New("not select in do Query")
	}
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if placeHolderNum != argCnt {
		return errors.New("argcnt mismatch")
	}
	return nil
}

func handleQueryRowContext(stmt sqlAST.StmtNode, argCnt int) error {
	ss, ok := stmt.(*sqlAST.SelectStmt)
	if ok {
		return errors.New("not select in do Query")
	}
	placeHolderNum := 0
	if ss.Where != nil {
		placeHolderNum = calcWherePlaceHolderNum(ss.Where.(*sqlAST.BinaryOperationExpr), placeHolderNum)
	}
	if placeHolderNum != argCnt {
		return errors.New("argcnt mismatch")
	}
	return nil
}

func handleExec(stmt sqlAST.StmtNode, argCnt int) error {
	switch t := stmt.(type) {
	case *sqlAST.InsertStmt:
		return handleInsert(t, argCnt)
	case *sqlAST.DeleteStmt:
		return handleDelete(t, argCnt)
	case *sqlAST.UpdateStmt:
		return handleUpdate(t, argCnt)
	default:
		panic(fmt.Sprintf("unexpected stmt:%s", stmt.Text()))
	}
}

func calcWherePlaceHolderNum(stmt *sqlAST.BinaryOperationExpr, cur int) int {
	switch t := stmt.L.(type) {
	case *sqlAST.BinaryOperationExpr:
		cur = calcWherePlaceHolderNum(t, cur)
	case *driver.ParamMarkerExpr:
		cur++
	default:
	}
	switch t := stmt.R.(type) {
	case *sqlAST.BinaryOperationExpr:
		cur = calcWherePlaceHolderNum(t, cur)
	case *driver.ParamMarkerExpr:
		cur++
	default:
	}
	return cur
}
