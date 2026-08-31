package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// messageMarker opts a message with no serialized fields into generation.
const messageMarker = "wiregen:message"

// message is one struct the generator emits a codec for.
type message struct {
	name   string
	fields []field
}

// loadPackage reads the Go source in dir and returns the package name and every
// message it declares, sorted by name. A struct is a message if it has at least
// one proto tagged field, or carries the wiregen:message marker comment.
func loadPackage(dir string) (string, []message, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}
	fset := token.NewFileSet()
	pkg := ""
	var msgs []message
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return "", nil, err
		}
		if pkg == "" {
			pkg = f.Name.Name
		} else if pkg != f.Name.Name {
			return "", nil, fmt.Errorf("%v: package %v does not match %v", name, f.Name.Name, pkg)
		}
		found, err := messages(f)
		if err != nil {
			return "", nil, fmt.Errorf("%v: %w", name, err)
		}
		msgs = append(msgs, found...)
	}
	slices.SortFunc(msgs, func(a, b message) int {
		return strings.Compare(a.name, b.name)
	})
	return pkg, msgs, nil
}

// messages returns the messages declared in one file.
func messages(f *ast.File) ([]message, error) {
	var msgs []message
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			fields, err := protoFields(st)
			if err != nil {
				return nil, fmt.Errorf("%v: %w", ts.Name.Name, err)
			}
			if len(fields) == 0 && !marked(gd, ts) {
				continue
			}
			msgs = append(msgs, message{
				name:   ts.Name.Name,
				fields: fields,
			})
		}
	}
	return msgs, nil
}

// marked reports whether a type carries the marker comment, either on the type
// or on a single type declaration wrapping it.
func marked(gd *ast.GenDecl, ts *ast.TypeSpec) bool {
	for _, doc := range []*ast.CommentGroup{ts.Doc, gd.Doc} {
		if doc == nil {
			continue
		}
		for _, c := range doc.List {
			if strings.TrimSpace(strings.TrimPrefix(c.Text, "//")) == messageMarker {
				return true
			}
		}
	}
	return false
}

// protoFields returns the fields of a struct that carry a proto tag, i.e. the
// fields that are serialized on the wire, in declaration order.
func protoFields(st *ast.StructType) ([]field, error) {
	var fields []field
	for _, f := range st.Fields.List {
		if f.Tag == nil {
			continue
		}
		value, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			return nil, err
		}
		tag, ok := reflect.StructTag(value).Lookup("proto")
		if !ok {
			continue
		}
		if len(f.Names) == 0 {
			return nil, fmt.Errorf("embedded field with a proto tag")
		}
		for _, name := range f.Names {
			parsed, err := parseField(name.Name, tag, elemName(f.Type))
			if err != nil {
				return nil, err
			}
			fields = append(fields, parsed)
		}
	}
	return fields, nil
}

// elemName is the name of a slice's element type, and empty for every other
// type.
func elemName(expr ast.Expr) string {
	arr, ok := expr.(*ast.ArrayType)
	if !ok || arr.Len != nil {
		return ""
	}
	return typeName(arr.Elt)
}

func typeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.StarExpr:
		return typeName(t.X)
	}
	return ""
}
