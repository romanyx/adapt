package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/alecthomas/template"
	"github.com/fatih/camelcase"
	"github.com/pkg/errors"
)

const (
	usg = `mockf [package] <interface>
mockf generates type func to implement interface.
Examples:
mockf io Reader
mockf iface
`

	stub = `type {{.LowerName}}Func func({{range .Params}}{{if .Variadic}}...{{end}}{{.Type}}, {{end}}) ({{range .Res}}{{.Type}}, {{end}})

func (f {{.LowerName}}Func) {{.MName}}({{range .Params}}{{.Name}} {{if .Variadic}}...{{end}}{{.Type}}, {{end}}) ({{range .Res}}{{.Name}} {{.Type}}, {{end}}) {
	{{if .HasReturn}}return {{end}}f({{range .Params}}{{.Name}}{{if .Variadic}}...{{end}}, {{end}})
}
`
)

var (
	tmpl = template.Must(template.New("test").Parse(stub))
)

func main() {
	flag.Parse()

	// mockf Interface
	if len(flag.Args()) < 2 {
		if len(flag.Args()) < 1 {
			usage()
		}

		pwd, err := os.Getwd()
		if err != nil {
			exit(err)
		}

		i := iface{
			Name: flag.Arg(0),
		}
		if err := parseDir(pwd, &i); err != nil {
			exit(err)
		}
		printIface(&i)

		return
	}

	// mockf pkg Interface
	i := iface{
		Name: flag.Arg(1),
	}
	if err := reflect(flag.Arg(0), &i); err != nil {
		exit(err)
	}
	printIface(&i)
}

// printIface prints given interface
// into StdOut.
func printIface(i *iface) {
	var buf bytes.Buffer
	tmpl.Execute(&buf, i)

	pretty, err := format.Source(buf.Bytes())
	if err != nil {
		exit(err)
	}

	fmt.Print(string(pretty))
}

// exit exits program with error.
func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}

// usage prints program usage.
func usage() {
	fmt.Fprint(os.Stderr, usg)
	os.Exit(2)
}

// iface represents interface declaration.
type iface struct {
	Name   string
	MName  string
	Params []param
	Res    []param
}

// HasReturn returns when iface
// has result parameters.
func (i iface) HasReturn() bool {
	return len(i.Res) > 0
}

// LowerName return iface name
// with lowercased first letter.
func (i iface) LowerName() string {
	l := []rune(i.Name)
	l[0] = unicode.ToLower(l[0])
	return string(l)
}

type param struct {
	Name     string
	Type     string
	Variadic bool
}

// parseDir parses interface from
// given directory.
func parseDir(path string, i *iface) error {
	bPkg, err := build.ImportDir(path, 0)
	if err != nil {
		return errors.Wrap(err, "import dir")
	}

	if err := fillInterface(bPkg, i); err != nil {
		return errors.Wrap(err, "fill interface")
	}

	return nil
}

// reflect parses interface from
// given package.
func reflect(pkg string, i *iface) error {
	bPkg, err := build.Import(pkg, "", 0)
	if err != nil {
		return errors.Wrapf(err, "couldn't find package %s", pkg)
	}

	if err := fillInterface(bPkg, i); err != nil {
		return errors.Wrap(err, "fill interface")
	}

	return nil
}

// fillInterface fills interface fields
// from given package.
func fillInterface(bPkg *build.Package, i *iface) error {
	p := pkg{
		bPkg: bPkg,
		fSet: token.NewFileSet(),
	}

	spec, err := specFromPackage(&p, i.Name)
	if err != nil {
		return errors.Wrap(err, "find interface")
	}

	idecl, ok := spec.Type.(*ast.InterfaceType)
	if !ok {
		return errors.Errorf("not an interface: %s", spec.Name)
	}

	if idecl.Methods == nil {
		return errors.Errorf("empty interface: %s", spec.Name)
	}

	methods := idecl.Methods.List
	if len(methods) > 1 {
		return errors.Errorf("only single method interfaces is supported")
	}

	meth := methods[0]
	if len(meth.Names) == 0 {
		return errors.Errorf("embedded interface not supported: %s", meth.Type)
	}

	fdecl(&p, meth, i)

	return nil
}

// pkg is a parsed build.Package.
type pkg struct {
	bPkg *build.Package
	fSet *token.FileSet
}

// specFormPackage locates the *ast.TypeSpec
// for type iName in the import path.
func specFromPackage(p *pkg, iName string) (*ast.TypeSpec, error) {
	for _, file := range p.bPkg.GoFiles {
		f, err := parser.ParseFile(p.fSet, filepath.Join(p.bPkg.Dir, file), nil, 0)
		if err != nil {
			continue
		}

		for _, decl := range f.Decls {
			decl, ok := decl.(*ast.GenDecl)
			if !ok || decl.Tok != token.TYPE {
				continue
			}
			for _, spec := range decl.Specs {
				spec := spec.(*ast.TypeSpec)
				if spec.Name.Name != iName {
					continue
				}

				return spec, nil
			}
		}
	}

	return nil, errors.Errorf("type %s not found in: %s", iName, p.bPkg.Name)
}

// fdecl fills MName, Params and Res fields from
// given iface from *ast.Field.
func fdecl(p *pkg, f *ast.Field, i *iface) error {
	i.MName = f.Names[0].Name
	typ, ok := f.Type.(*ast.FuncType)
	if !ok {
		return errors.Errorf("not a func type: %s", i.MName)
	}
	check := make(map[string]bool)
	if typ.Params != nil {
		for _, field := range typ.Params.List {
			i.Params = append(i.Params, p.params(field, check)...)
		}
	}
	if typ.Results != nil {
		for _, field := range typ.Results.List {
			i.Res = append(i.Res, p.resultParams(field))
		}
	}

	return nil
}

// resultParams returns params from given field.
func (p *pkg) resultParams(field *ast.Field) param {
	typ := fullType(p, field.Type)
	return param{Type: typ}
}

// params returns params form given field.
// It checks that params names won't be the same.
func (p *pkg) params(field *ast.Field, check map[string]bool) []param {
	var pms []param
	typ := fullType(p, field.Type)
	for _, name := range field.Names {
		pms = append(pms, param{Name: name.Name, Type: typ})
	}

	// Handle anonymous params
	if len(pms) == 0 {
		var v bool
		if strings.HasPrefix(typ, "...") {
			typ = strings.TrimPrefix(typ, "...")
			v = true
		}

		var name string
		parts := strings.Split(typ, ".")
		name = parts[0]
		if len(parts) > 1 {
			name = parts[1]
		}

		splitted := camelcase.Split(name)
		name = generateName(splitted, check, 1)
		check[name] = true

		return []param{param{Name: name, Type: typ, Variadic: v}}
	}

	return pms
}

// fullType returns the fully qualified type of e.
// Examples, assuming package net/http:
// 	fullType(int) => "int"
// 	fullType(Handler) => "http.Handler"
// 	fullType(io.Reader) => "io.Reader"
// 	fullType(*Request) => "*http.Request"
func fullType(p *pkg, e ast.Expr) string {
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			// Using typeSpec instead of IsExported here would be
			// more accurate, but it'd be crazy expensive, and if
			// the type isn't exported, there's no point trying
			// to implement it anyway.
			if n.IsExported() {
				n.Name = p.bPkg.Name + "." + n.Name
			}
		case *ast.SelectorExpr:
			return false
		}
		return true
	})

	var buf bytes.Buffer
	printer.Fprint(&buf, p.fSet, e)
	return buf.String()
}

// generateName will generate and return name for given
// words. It will take first n letters from each words
// and will concat it, if name is already exits in check
// map it will call itself recursively with n+1. if n
// will be more than letters count in word it will
// just return lowercase word.
func generateName(words []string, check map[string]bool, n int) string {
	var r string
	for _, w := range words {
		ltrs := strings.Split(w, "")
		if len(ltrs) < n {
			return strings.ToLower(w)
		}

		r += strings.ToLower(strings.Join(ltrs[0:n], ""))
	}

	if check[r] {
		r = generateName(words, check, n+1)
	}

	return r
}
