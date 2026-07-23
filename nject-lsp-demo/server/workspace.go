package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"golang.org/x/tools/go/packages"
)

const njectPackagePath = "github.com/muir/nject/v2"

type sourceFile struct {
	pkg      *packages.Package
	syntax   *ast.File
	filename string
	content  []byte
}

type expressionRef struct {
	pkg  *packages.Package
	expr ast.Expr
}

type parameterOwner struct {
	functionID string
	index      int
	variadic   bool
}

type callSite struct {
	pkg  *packages.Package
	call *ast.CallExpr
}

type functionNode struct {
	id        string
	info      functionInfo
	signature *types.Signature
}

type selectedParameter struct {
	function functionNode
	typ      types.Type
}

type analysisWorkspace struct {
	overlays     map[string][]byte
	packages     []*packages.Package
	files        map[string]*sourceFile
	functions    map[string]functionNode
	initializers map[*types.Var]expressionRef
	parameters   map[*types.Var]parameterOwner
	calls        map[string][]callSite
}

func loadAnalysisWorkspace(
	ctx context.Context,
	filename string,
	overlays map[string][]byte,
) (*analysisWorkspace, error) {
	root, err := moduleRoot(filename)
	if err != nil {
		return nil, err
	}

	metadata, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     root,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedModule,
		Overlay: overlays,
	}, "./...")
	if err != nil {
		return nil, fmt.Errorf("load workspace metadata: %w", err)
	}

	var patterns []string
	targetCanUseNject := false
	for _, pkg := range metadata {
		if !packageDependsOn(pkg, njectPackagePath, make(map[string]bool)) {
			continue
		}
		patterns = append(patterns, pkg.PkgPath)
		if packageContainsFile(pkg, filename) {
			targetCanUseNject = true
		}
	}
	if !targetCanUseNject || len(patterns) == 0 {
		return nil, nil
	}
	slices.Sort(patterns)
	patterns = slices.Compact(patterns)

	loaded, err := packages.Load(&packages.Config{
		Context: ctx,
		Dir:     root,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes |
			packages.NeedModule,
		Overlay: overlays,
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load nject-related packages: %w", err)
	}

	workspace := &analysisWorkspace{
		overlays:     overlays,
		packages:     loaded,
		files:        make(map[string]*sourceFile),
		functions:    make(map[string]functionNode),
		initializers: make(map[*types.Var]expressionRef),
		parameters:   make(map[*types.Var]parameterOwner),
		calls:        make(map[string][]callSite),
	}
	if err := workspace.index(); err != nil {
		return nil, err
	}
	return workspace, nil
}

func moduleRoot(filename string) (string, error) {
	directory := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("no go.mod found for %s", filename)
		}
		directory = parent
	}
}

func packageContainsFile(pkg *packages.Package, filename string) bool {
	for _, candidate := range append(pkg.GoFiles, pkg.CompiledGoFiles...) {
		absolute, err := filepath.Abs(candidate)
		if err == nil && absolute == filename {
			return true
		}
	}
	return false
}

func packageDependsOn(pkg *packages.Package, importPath string, seen map[string]bool) bool {
	if pkg == nil || seen[pkg.ID] {
		return false
	}
	seen[pkg.ID] = true
	if pkg.PkgPath == importPath {
		return true
	}
	for _, imported := range pkg.Imports {
		if packageDependsOn(imported, importPath, seen) {
			return true
		}
	}
	return false
}

func (w *analysisWorkspace) index() error {
	for _, pkg := range w.packages {
		for index, syntax := range pkg.Syntax {
			if index >= len(pkg.CompiledGoFiles) {
				continue
			}
			filename, err := filepath.Abs(pkg.CompiledGoFiles[index])
			if err != nil {
				continue
			}
			content, ok := w.overlays[filename]
			if !ok {
				content, err = os.ReadFile(filename)
				if err != nil {
					return fmt.Errorf("read %s: %w", filename, err)
				}
			}
			w.files[filename] = &sourceFile{
				pkg:      pkg,
				syntax:   syntax,
				filename: filename,
				content:  content,
			}
			w.indexDeclarations(pkg, syntax)
		}
	}

	for _, source := range w.files {
		ast.Inspect(source.syntax, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.AssignStmt:
				w.indexAssignment(source.pkg, typed)
			case *ast.CallExpr:
				if function := calledFunction(source.pkg.TypesInfo, typed.Fun); function != nil {
					id := functionObjectID(function)
					w.calls[id] = append(w.calls[id], callSite{
						pkg:  source.pkg,
						call: typed,
					})
				}
			}
			return true
		})
	}
	return nil
}

func (w *analysisWorkspace) indexDeclarations(pkg *packages.Package, syntax *ast.File) {
	for _, declaration := range syntax.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			function, ok := pkg.TypesInfo.Defs[typed.Name].(*types.Func)
			if !ok {
				continue
			}
			signature, ok := function.Type().(*types.Signature)
			if !ok {
				continue
			}
			id := functionObjectID(function)
			if info, ok := w.functionInfoAt(pkg, function.Name(), typed.Name.Pos()); ok {
				w.functions[id] = functionNode{id: id, info: info, signature: signature}
			}
			w.indexParameters(pkg, typed, id, signature)
		case *ast.GenDecl:
			for _, specification := range typed.Specs {
				value, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, name := range value.Names {
					if index >= len(value.Values) {
						continue
					}
					variable, ok := pkg.TypesInfo.Defs[name].(*types.Var)
					if ok {
						w.initializers[variable] = expressionRef{pkg: pkg, expr: value.Values[index]}
					}
				}
			}
		}
	}
}

func (w *analysisWorkspace) indexParameters(
	pkg *packages.Package,
	declaration *ast.FuncDecl,
	functionID string,
	signature *types.Signature,
) {
	if declaration.Type.Params == nil {
		return
	}
	parameterIndex := 0
	for _, field := range declaration.Type.Params.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for _, name := range field.Names {
			variable, ok := pkg.TypesInfo.Defs[name].(*types.Var)
			if ok {
				w.parameters[variable] = parameterOwner{
					functionID: functionID,
					index:      parameterIndex,
					variadic:   signature.Variadic() && parameterIndex == signature.Params().Len()-1,
				}
			}
			parameterIndex++
		}
		if len(field.Names) == 0 {
			parameterIndex += count - 1
		}
	}
}

func (w *analysisWorkspace) indexAssignment(pkg *packages.Package, assignment *ast.AssignStmt) {
	for index, left := range assignment.Lhs {
		if index >= len(assignment.Rhs) {
			continue
		}
		identifier, ok := left.(*ast.Ident)
		if !ok {
			continue
		}
		var variable *types.Var
		if defined, ok := pkg.TypesInfo.Defs[identifier].(*types.Var); ok {
			variable = defined
		} else if used, ok := pkg.TypesInfo.Uses[identifier].(*types.Var); ok {
			variable = used
		}
		if variable != nil {
			w.initializers[variable] = expressionRef{pkg: pkg, expr: assignment.Rhs[index]}
		}
	}
}

func (w *analysisWorkspace) selected(
	filename string,
	line uint32,
	character uint32,
) (*selectedParameter, error) {
	source := w.files[filename]
	if source == nil {
		return nil, nil
	}
	offset, ok := offsetForPosition(source.content, line, character)
	if !ok {
		return nil, nil
	}
	tokenFile := source.pkg.Fset.File(source.syntax.Pos())
	if tokenFile == nil || offset > tokenFile.Size() {
		return nil, nil
	}
	position := tokenFile.Pos(offset)

	for _, declaration := range source.syntax.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || !positionInSignature(function, position) || function.Type.Params == nil {
			continue
		}
		object, ok := source.pkg.TypesInfo.Defs[function.Name].(*types.Func)
		if !ok {
			return nil, nil
		}
		node, ok := w.functions[functionObjectID(object)]
		if !ok {
			return nil, nil
		}
		for _, field := range function.Type.Params.List {
			if !positionInField(field, position) {
				continue
			}
			typ := source.pkg.TypesInfo.TypeOf(field.Type)
			if typ == nil {
				return nil, nil
			}
			return &selectedParameter{function: node, typ: typ}, nil
		}
	}
	return nil, nil
}

func (w *analysisWorkspace) functionInfoAt(
	pkg *packages.Package,
	name string,
	position token.Pos,
) (functionInfo, bool) {
	tokenFile := pkg.Fset.File(position)
	if tokenFile == nil {
		return functionInfo{}, false
	}
	filename, err := filepath.Abs(tokenFile.Name())
	if err != nil {
		return functionInfo{}, false
	}
	content, ok := w.overlays[filename]
	if !ok {
		content, err = os.ReadFile(filename)
		if err != nil {
			return functionInfo{}, false
		}
	}
	line, character, ok := positionForOffset(content, tokenFile.Offset(position))
	if !ok {
		return functionInfo{}, false
	}
	return functionInfo{
		Name:      name,
		Filename:  filename,
		Line:      line,
		Character: character,
	}, true
}

func functionObjectID(function *types.Func) string {
	return types.ObjectString(function, func(pkg *types.Package) string {
		return strconv.Quote(pkg.Path())
	})
}

func calledFunction(info *types.Info, expression ast.Expr) *types.Func {
	switch typed := expression.(type) {
	case *ast.Ident:
		function, _ := info.Uses[typed].(*types.Func)
		return function
	case *ast.SelectorExpr:
		function, _ := info.Uses[typed.Sel].(*types.Func)
		return function
	case *ast.IndexExpr:
		return calledFunction(info, typed.X)
	case *ast.IndexListExpr:
		return calledFunction(info, typed.X)
	case *ast.ParenExpr:
		return calledFunction(info, typed.X)
	default:
		return nil
	}
}
