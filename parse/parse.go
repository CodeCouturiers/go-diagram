package parse

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Type struct {
	Literal string   `json:"literal"`
	Structs []string `json:"structs"`
}

type ClientStruct struct {
	Packages        []Package  `json:"packages"`
	Edges           []Edge     `json:"edges"`
	GlobalFunctions []Function `json:"globalFunctions"`
}

type Package struct {
	Name  string `json:"name"`
	Files []File `json:"files"`
}

type File struct {
	Name    string   `json:"name"`
	Structs []Struct `json:"structs"`
}

type Struct struct {
	Name    string   `json:"name"`
	Fields  []Field  `json:"fields"`
	Methods []Method `json:"methods"`
}

type Field struct {
	Name string `json:"name"`
	Type Type   `json:"type"`
}

type Method struct {
	Name       string `json:"name"`
	ReturnType []Type `json:"returnType"`
}

type Function struct {
	Name       string      `json:"name"`
	Package    string      `json:"package"`
	File       string      `json:"file"`
	Parameters []Parameter `json:"parameters"`
	ReturnType []Type      `json:"returnType"`
}

type Parameter struct {
	Name string `json:"name"`
	Type Type   `json:"type"`
}

type Node struct {
	FieldTypeName string `json:"fieldTypeName"`
	StructName    string `json:"structName"`
	PackageName   string `json:"packageName"`
	FileName      string `json:"fileName"`
}

type Edge struct {
	To   *Node `json:"to"`
	From *Node `json:"from"`
}

func GetStructsFile(fset *token.FileSet, f *ast.File, fname string, packageName string) (File, []Edge, []Function) {
	structs := []Struct{}
	edges := []Edge{}
	globalFunctions := []Function{}

	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.GenDecl:
			if decl.Tok == token.TYPE {
				for _, s := range decl.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok {
						if st, ok := ts.Type.(*ast.StructType); ok {
							fields := []Field{}
							for _, field := range st.Fields.List {
								for _, name := range field.Names {
									var buf bytes.Buffer
									if err := format.Node(&buf, fset, field.Type); err != nil {
										panic(err)
									}
									stname, toNodes := GetTypes(field.Type, packageName)
									fieldtype := Type{Literal: string(buf.Bytes()), Structs: stname}
									fi := Field{Name: name.Name, Type: fieldtype}
									fields = append(fields, fi)

									for _, toNode := range toNodes {
										edges = append(edges, Edge{
											From: &Node{
												FieldTypeName: name.Name,
												StructName:    ts.Name.Name,
												FileName:      fname,
												PackageName:   packageName,
											},
											To: toNode,
										})
									}
								}
							}
							structs = append(structs, Struct{Name: ts.Name.Name, Fields: fields})
						}
					}
				}
			}
		case *ast.FuncDecl:
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				// This is a method
				recvType := decl.Recv.List[0].Type
				var structName string
				if starExpr, ok := recvType.(*ast.StarExpr); ok {
					structName = starExpr.X.(*ast.Ident).Name
				} else {
					structName = recvType.(*ast.Ident).Name
				}

				method := Method{
					Name:       decl.Name.Name,
					ReturnType: parseReturnTypes(decl.Type.Results),
				}

				// Find the struct and add the method
				for i, s := range structs {
					if s.Name == structName {
						structs[i].Methods = append(structs[i].Methods, method)
						break
					}
				}
			} else {
				// This is a global function
				globalFunctions = append(globalFunctions, Function{
					Name:       decl.Name.Name,
					Package:    packageName,
					File:       fname,
					Parameters: parseParameters(decl.Type.Params),
					ReturnType: parseReturnTypes(decl.Type.Results),
				})
			}
		}
	}

	return File{Name: fname, Structs: structs}, edges, globalFunctions
}

func GetFileName(toNode *Node, pkgs []Package) string {
	for _, pkg := range pkgs {
		if pkg.Name == toNode.PackageName {
			for _, file := range pkg.Files {
				for _, st := range file.Structs {
					if st.Name == toNode.StructName {
						return file.Name
					}
				}
			}
		}
	}
	fmt.Println("Matching file not found for struct", toNode.StructName, "(probably a library package)")
	return ""
}

func getPackagesEdgesDirName(fset *token.FileSet, packages map[string]*ast.Package) ([]Package, []Edge, []Function, error) {
	var pkgs []Package
	var edges []Edge
	var globalFunctions []Function

	for packagename, packageval := range packages {
		files := []File{}
		for fname, f := range packageval.Files {
			newfile, newedges, newFunctions := GetStructsFile(fset, f, fname, packagename)
			files = append(files, newfile)
			edges = append(edges, newedges...)
			globalFunctions = append(globalFunctions, newFunctions...)
			log.Printf("Parsed file: %s", fname)
		}
		pkgs = append(pkgs, Package{Name: packagename, Files: files})
	}

	return pkgs, edges, globalFunctions, nil
}

func parseDirectory(path string) (map[string]*ast.Package, error) {
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("error parsing directory %s: %w", path, err)
	}

	// Логгируем файлы, которые не были спарсены
	err = filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() && (strings.Contains(path, ".git") || strings.Contains(path, "node_modules")) {
			return nil
		}
		if f.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		_, ok := packages[filepath.Base(filepath.Dir(path))]
		if !ok {
			log.Printf("Skipped file: %s", path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return packages, nil
}

func GetStructsDirName(path string) (*ClientStruct, map[string]*ast.Package, error) {
	var directories []string
	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() && (strings.Contains(path, ".git") || strings.Contains(path, "node_modules")) {
			return nil
		}
		if f.IsDir() {
			directories = append(directories, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error walking directory: %w", err)
	}

	var packages []Package
	var edges []Edge
	var globalFunctions []Function
	pkgmap := map[string]*ast.Package{}
	fset := token.NewFileSet()

	for _, directory := range directories {
		dirPackages, err := parseDirectory(directory)
		if err != nil {
			return nil, nil, err
		}

		newpackages, newedges, newFunctions, err := getPackagesEdgesDirName(fset, dirPackages)
		if err != nil {
			return nil, nil, err
		}

		packages = append(packages, newpackages...)
		edges = append(edges, newedges...)
		globalFunctions = append(globalFunctions, newFunctions...)
		for k, v := range dirPackages {
			pkgmap[k] = v
		}
	}

	// Fill in filenames for edges
	validedges := []Edge{}
	for _, edge := range edges {
		if name := GetFileName(edge.To, packages); name != "" {
			edge.To.FileName = name
			validedges = append(validedges, edge)
		}
	}

	return &ClientStruct{Packages: packages, Edges: validedges, GlobalFunctions: globalFunctions}, pkgmap, nil
}

func isPrimitive(name string) bool {
	primitives := map[string]bool{
		"bool":       true,
		"byte":       true,
		"complex64":  true,
		"complex128": true,
		"error":      true,
		"float32":    true,
		"float64":    true,
		"int":        true,
		"int8":       true,
		"int16":      true,
		"int32":      true,
		"int64":      true,
		"rune":       true,
		"string":     true,
		"uint":       true,
		"uint8":      true,
		"uint16":     true,
		"uint32":     true,
		"uint64":     true,
		"uintptr":    true,
	}
	return primitives[name]
}

func GetTypes(node ast.Expr, packageName string) ([]string, []*Node) {
	var structs []string
	var nodes []*Node

	var extractType func(ast.Expr)
	extractType = func(expr ast.Expr) {
		switch t := expr.(type) {
		case *ast.Ident:
			name := t.Name
			if !isPrimitive(name) {
				structs = append(structs, name)
				nodes = append(nodes, &Node{StructName: name, PackageName: packageName})
			}
		case *ast.SelectorExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				structs = append(structs, ident.Name+"."+t.Sel.Name)
				nodes = append(nodes, &Node{StructName: t.Sel.Name, PackageName: ident.Name})
			}
		case *ast.StarExpr:
			extractType(t.X)
		case *ast.ArrayType:
			extractType(t.Elt)
		case *ast.MapType:
			extractType(t.Key)
			extractType(t.Value)
		case *ast.StructType:
			for _, field := range t.Fields.List {
				extractType(field.Type)
			}
		case *ast.InterfaceType:
			structs = append(structs, "interface{}")
		case *ast.FuncType:
			structs = append(structs, "func")
		case *ast.ChanType:
			extractType(t.Value)
			structs = append(structs, "chan")
		}
	}

	extractType(node)
	return structs, nodes
}

func parseParameters(fieldList *ast.FieldList) []Parameter {
	var params []Parameter
	if fieldList == nil {
		return params
	}
	for _, field := range fieldList.List {
		fieldType := parseTypeToType(field.Type)
		for _, name := range field.Names {
			params = append(params, Parameter{
				Name: name.Name,
				Type: fieldType,
			})
		}
	}
	return params
}

func parseReturnTypes(fieldList *ast.FieldList) []Type {
	var types []Type
	if fieldList == nil {
		return types
	}
	for _, field := range fieldList.List {
		types = append(types, parseTypeToType(field.Type))
	}
	return types
}

func parseTypeToType(expr ast.Expr) Type {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), expr); err != nil {
		log.Printf("Error formatting type: %v", err)
		return Type{Literal: "error"}
	}
	return Type{Literal: buf.String()}
}

func WriteClientPackages(pkgs map[string]*ast.Package, clientpackages []Package) error {
	var err error
	for _, clientpackage := range clientpackages {
		for _, clientfile := range clientpackage.Files {
			packagename := clientpackage.Name
			packageast := pkgs[packagename]
			// Get the AST with the matching file name
			f := packageast.Files[clientfile.Name]

			if f == nil {
				fmt.Println("Couldn't find", packagename, packageast.Files, clientfile.Name)
			}
			// Update the AST with the values from the client
			f, err = clientFileToAST(clientfile, f)
			if err != nil {
				return err
			}
			writeFileAST(clientfile.Name, f)
		}
	}
	return nil
}

func writeFileAST(filepath string, f *ast.File) {
	fset := token.NewFileSet()
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		panic(err)
	}
	err := ioutil.WriteFile(filepath, buf.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
}

func clientFileToAST(clientfile File, f *ast.File) (*ast.File, error) {
	var newDecls []ast.Decl
	if len(f.Decls) > 0 {
		newDecls = []ast.Decl{f.Decls[0]}
	}

	f.Decls = removeStructDecls(f.Decls)
	newStructs, err := clientFileToDecls(clientfile)
	if err != nil {
		return nil, err
	}

	// Assume import is the first decl. Add typedefs after that
	newDecls = append(newDecls, newStructs...)
	if len(f.Decls) > 0 {
		newDecls = append(newDecls, f.Decls[1:]...)
	} else {
		newDecls = append(newDecls, f.Decls...)
	}
	f.Decls = newDecls
	return f, nil
}

func clientFileToDecls(clientfile File) ([]ast.Decl, error) {
	var decls []ast.Decl

	for _, clientstruct := range clientfile.Structs {
		// Create struct declaration
		structDecl := &ast.GenDecl{
			Tok: token.TYPE,
			Specs: []ast.Spec{
				&ast.TypeSpec{
					Name: ast.NewIdent(clientstruct.Name),
					Type: &ast.StructType{
						Fields: &ast.FieldList{},
					},
				},
			},
		}

		for _, clientfield := range clientstruct.Fields {
			fieldType, err := parseType(clientfield.Type.Literal)
			if err != nil {
				return nil, fmt.Errorf("error parsing field type: %w", err)
			}

			field := &ast.Field{
				Names: []*ast.Ident{ast.NewIdent(clientfield.Name)},
				Type:  fieldType,
			}

			structDecl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType).Fields.List = append(
				structDecl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType).Fields.List,
				field,
			)
		}

		decls = append(decls, structDecl)

		// Create method declarations
		for _, clientmethod := range clientstruct.Methods {
			methodDecl := &ast.FuncDecl{
				Recv: &ast.FieldList{
					List: []*ast.Field{
						{
							Names: []*ast.Ident{ast.NewIdent("s")},
							Type:  &ast.StarExpr{X: ast.NewIdent(clientstruct.Name)},
						},
					},
				},
				Name: ast.NewIdent(clientmethod.Name),
				Type: &ast.FuncType{
					Params:  &ast.FieldList{},
					Results: &ast.FieldList{},
				},
			}

			// Add return types
			for _, returnType := range clientmethod.ReturnType {
				retType, err := parseType(returnType.Literal)
				if err != nil {
					return nil, fmt.Errorf("error parsing return type: %w", err)
				}
				methodDecl.Type.Results.List = append(methodDecl.Type.Results.List, &ast.Field{Type: retType})
			}

			decls = append(decls, methodDecl)
		}
	}

	return decls, nil
}

func parseType(typeStr string) (ast.Expr, error) {
	expr, err := parser.ParseExpr(typeStr)
	if err != nil {
		return nil, err
	}
	return expr, nil
}

func removeStructDecls(decls []ast.Decl) []ast.Decl {
	var newDecls []ast.Decl
	for _, decl := range decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				var newSpecs []ast.Spec
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if _, isStruct := ts.Type.(*ast.StructType); !isStruct {
							newSpecs = append(newSpecs, spec)
						}
					} else {
						newSpecs = append(newSpecs, spec)
					}
				}
				if len(newSpecs) > 0 {
					d.Specs = newSpecs
					newDecls = append(newDecls, d)
				}
			} else {
				newDecls = append(newDecls, decl)
			}
		default:
			newDecls = append(newDecls, decl)
		}
	}
	return newDecls
}
