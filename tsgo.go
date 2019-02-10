package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"path/filepath"
)

func typeContainsPointer(t types.Type) (bool, types.Type) {
	switch t := t.(type) {
	case *types.Array:
		return true, t
	case *types.Basic:
		if t.Kind() != types.UnsafePointer {
			return true, t
		}
	case *types.Chan:
		return false, nil
	case *types.Interface:
		return false, nil
	case *types.Map:
		return true, t
	case *types.Named:
		return typeContainsPointer(t.Underlying())
	case *types.Pointer:
		return true, t
	case *types.Slice:
		return true, t
	case *types.Struct:
		numFields := t.NumFields()
		for i := 0; i < numFields; i++ {
			fieldType := t.Field(i).Type()
			if contains, subType := typeContainsPointer(fieldType); contains {
				return true, subType
			}
		}
		return false, nil
	// case *types.Nil:
	// 	return false
	// case *types.Object:
	// 	return true
	}
	return true, t
}

func stringifyNode(fset *token.FileSet, node ast.Node) string {
	buffer := bytes.Buffer{}
	err := printer.Fprint(&buffer, fset, node)
	if err != nil {
		panic(err)
	}
	return buffer.String()
}

func printError(fset *token.FileSet, node ast.Node, message string, t types.Type) {
	var annotation interface{}
	if t != nil {
		annotation = t
	} else {
		annotation = stringifyNode(fset, node)
	}
	fmt.Printf("%s:warning: %s (%v)\n", fset.Position(node.Pos()), message, annotation)
}

type visitor struct{
	fset *token.FileSet
	info types.Info
	isInsideFunction bool
}

func (v *visitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.SendStmt:
		contains, pointerType := typeContainsPointer(v.info.TypeOf(n.Value))
		if contains {
			printError(v.fset, n, "sending pointer type over a channel", pointerType)
		}
	case *ast.GoStmt:
		contains, pointerType := typeContainsPointer(v.info.TypeOf(n.Call.Fun))
		if contains {
			printError(v.fset, n, "calling goroutine on a pointer type", pointerType)
		}
		for _, arg := range n.Call.Args {
			contains, pointerType := typeContainsPointer(v.info.TypeOf(arg))
			if contains {
				printError(v.fset, arg, "calling goroutine with a pointer type", pointerType)
			}
		}
	case *ast.GenDecl:
		if !v.isInsideFunction && n.Tok == token.VAR {
			for _, spec := range n.Specs {
				printError(v.fset, spec, "global var declared", nil)
			}
		}
	case *ast.FuncDecl:
		newVisitor := *v
		newVisitor.isInsideFunction = true
		return &newVisitor
	case *ast.Ident:
		// obj := v.info.ObjectOf(n)
		// switch obj := obj.(type) {
		// case *types.Var:
		// 	fmt.Printf("%s:warning: %s (%v)\n", v.fset.Position(n.Pos()), "name", obj.Name())
		// }
		// printError(v.fset, obj, "identifier", nil)
		// printError(v.fset, n, "identifier", nil)
	}
	return v
}

func main() {
	pkg, err := build.ImportDir("./", 0)
	if err != nil {
		panic(err)
	}

	type file struct {
		path string
		node *ast.File
	}
	fset := token.NewFileSet()
	files := make([]file, len(pkg.GoFiles))
	for i, path := range pkg.GoFiles {
		if pkg.Dir != "." {
			path = filepath.Join(pkg.Dir, path)
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			panic(err)
		}
		files[i] = file{
			path: path,
			node: f,
		}
	}

	cfg := types.Config{ Importer: importer.Default() }

	for _, f := range files {
		info := types.Info{
			Types: map[ast.Expr]types.TypeAndValue{},
			Defs: map[*ast.Ident]types.Object{},
			Uses: map[*ast.Ident]types.Object{},	
		}
		_, err = cfg.Check(f.path, fset, []*ast.File { f.node }, &info)
		if err != nil {
			panic(err)
		}

		ast.Walk(&visitor{
			fset: fset,
			info: info,
		}, f.node)
	}
}
