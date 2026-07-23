package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"

	"golang.org/x/tools/go/packages"
)

const maxProviderResolutionDepth = 64

func (w *analysisWorkspace) findChain(
	filename string,
	line uint32,
	character uint32,
) ([]functionInfo, error) {
	selected, err := w.selected(filename, line, character)
	if err != nil || selected == nil {
		return nil, err
	}

	seenResults := make(map[string]bool)
	var results []functionInfo
	addResult := func(function functionNode) {
		if seenResults[function.id] {
			return
		}
		seenResults[function.id] = true
		results = append(results, function.info)
	}
	for _, root := range w.njectRunCalls() {
		var providers []functionNode
		for index := 1; index < len(root.call.Args); index++ {
			providers = append(
				providers,
				w.resolveProvider(
					expressionRef{pkg: root.pkg, expr: root.call.Args[index]},
					make(map[string]bool),
					0,
				)...,
			)
		}

		for selectedIndex, provider := range providers {
			if provider.id != selected.function.id {
				continue
			}
			producerIndexes := make(map[int]bool)
			w.collectProducers(providers, selected.typ, selectedIndex, producerIndexes)

			ordered := make([]int, 0, len(producerIndexes))
			for index := range producerIndexes {
				ordered = append(ordered, index)
			}
			sort.Ints(ordered)
			for _, index := range ordered {
				addResult(providers[index])
			}
			addResult(selected.function)
		}
	}

	if len(results) == 0 {
		return nil, nil
	}
	return results, nil
}

func (w *analysisWorkspace) collectProducers(
	providers []functionNode,
	wanted types.Type,
	before int,
	found map[int]bool,
) {
	for index := before - 1; index >= 0; index-- {
		if !signatureProduces(providers[index].signature, wanted) {
			continue
		}
		if found[index] {
			return
		}
		found[index] = true
		for parameterIndex := 0; parameterIndex < providers[index].signature.Params().Len(); parameterIndex++ {
			w.collectProducers(
				providers,
				providers[index].signature.Params().At(parameterIndex).Type(),
				index,
				found,
			)
		}
		return
	}
}

func signatureProduces(signature *types.Signature, wanted types.Type) bool {
	if signature == nil {
		return false
	}
	for index := 0; index < signature.Results().Len(); index++ {
		if types.Identical(signature.Results().At(index).Type(), wanted) {
			return true
		}
	}
	return false
}

func (w *analysisWorkspace) njectRunCalls() []callSite {
	var roots []callSite
	filenames := make([]string, 0, len(w.files))
	for filename := range w.files {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	for _, filename := range filenames {
		source := w.files[filename]
		ast.Inspect(source.syntax, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			function := calledFunction(source.pkg.TypesInfo, call.Fun)
			if isNjectFunction(function, "Run", "MustRun") {
				roots = append(roots, callSite{pkg: source.pkg, call: call})
			}
			return true
		})
	}
	return roots
}

func (w *analysisWorkspace) resolveProvider(
	reference expressionRef,
	visited map[string]bool,
	depth int,
) []functionNode {
	if reference.expr == nil || reference.pkg == nil || depth > maxProviderResolutionDepth {
		return nil
	}
	key := fmt.Sprintf("%s:%d", reference.pkg.ID, reference.expr.Pos())
	if visited[key] {
		return nil
	}
	visited[key] = true
	defer delete(visited, key)

	switch expression := reference.expr.(type) {
	case *ast.ParenExpr:
		return w.resolveProvider(
			expressionRef{pkg: reference.pkg, expr: expression.X},
			visited,
			depth+1,
		)
	case *ast.Ident:
		if function, ok := reference.pkg.TypesInfo.Uses[expression].(*types.Func); ok {
			return w.nodesForFunction(function, reference)
		}
		if variable, ok := reference.pkg.TypesInfo.Uses[expression].(*types.Var); ok {
			if owner, exists := w.parameters[variable]; exists {
				return w.resolveParameter(owner, visited, depth+1)
			}
			if initializer, exists := w.initializers[variable]; exists {
				return w.resolveProvider(initializer, visited, depth+1)
			}
		}
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		if function := calledFunction(reference.pkg.TypesInfo, expression); function != nil {
			return w.nodesForFunction(function, reference)
		}
	case *ast.FuncLit:
		signature, ok := reference.pkg.TypesInfo.TypeOf(expression).(*types.Signature)
		if !ok {
			return nil
		}
		id := fmt.Sprintf("literal:%s:%d", reference.pkg.ID, expression.Pos())
		info, ok := w.functionInfoAt(reference.pkg, "anonymous function", expression.Type.Func)
		if !ok {
			return nil
		}
		return []functionNode{{id: id, info: info, signature: signature}}
	case *ast.CompositeLit:
		var providers []functionNode
		for _, element := range expression.Elts {
			value, ok := element.(ast.Expr)
			if !ok {
				continue
			}
			providers = append(
				providers,
				w.resolveProvider(
					expressionRef{pkg: reference.pkg, expr: value},
					visited,
					depth+1,
				)...,
			)
		}
		return providers
	case *ast.CallExpr:
		return w.resolveProviderCall(reference.pkg, expression, visited, depth+1)
	}
	return nil
}

func (w *analysisWorkspace) resolveProviderCall(
	pkg *packages.Package,
	call *ast.CallExpr,
	visited map[string]bool,
	depth int,
) []functionNode {
	function := calledFunction(pkg.TypesInfo, call.Fun)
	if function == nil || function.Pkg() == nil || function.Pkg().Path() != njectPackagePath {
		return nil
	}

	var expressions []ast.Expr
	switch function.Name() {
	case "Sequence", "Cluster":
		if len(call.Args) > 1 {
			expressions = call.Args[1:]
		}
	case "Provide":
		if len(call.Args) > 1 {
			expressions = call.Args[1:2]
		}
	case "Append":
		if selector, ok := unindexedExpression(call.Fun).(*ast.SelectorExpr); ok {
			expressions = append(expressions, selector.X)
		}
		if len(call.Args) > 1 {
			expressions = append(expressions, call.Args[1:]...)
		}
	case "Required", "Desired", "Shun", "Cacheable", "MustCache",
		"NotCacheable", "Memoize", "Singleton", "NonFinal", "Reorder",
		"Parallel", "OverridesError":
		if len(call.Args) > 0 {
			expressions = call.Args[:1]
		}
	default:
		// Generic decorators such as MustConsume[T] and Loose[T] also wrap
		// their first argument. Unknown dynamic nject APIs remain opaque.
		if len(call.Args) == 1 {
			expressions = call.Args[:1]
		}
	}

	var providers []functionNode
	for _, expression := range expressions {
		providers = append(
			providers,
			w.resolveProvider(expressionRef{pkg: pkg, expr: expression}, visited, depth)...,
		)
	}
	return providers
}

func (w *analysisWorkspace) resolveParameter(
	owner parameterOwner,
	visited map[string]bool,
	depth int,
) []functionNode {
	var providers []functionNode
	for _, site := range w.calls[owner.functionID] {
		if owner.index >= len(site.call.Args) {
			continue
		}
		end := owner.index + 1
		if owner.variadic {
			end = len(site.call.Args)
		}
		for _, argument := range site.call.Args[owner.index:end] {
			providers = append(
				providers,
				w.resolveProvider(
					expressionRef{pkg: site.pkg, expr: argument},
					visited,
					depth+1,
				)...,
			)
		}
	}
	return providers
}

func (w *analysisWorkspace) nodesForFunction(
	function *types.Func,
	reference expressionRef,
) []functionNode {
	id := functionObjectID(function)
	if node, ok := w.functions[id]; ok {
		return []functionNode{node}
	}
	signature, ok := function.Type().(*types.Signature)
	if !ok {
		return nil
	}
	info, ok := w.functionInfoAt(reference.pkg, function.Name(), reference.expr.Pos())
	if !ok {
		return nil
	}
	return []functionNode{{id: id, info: info, signature: signature}}
}

func isNjectFunction(function *types.Func, names ...string) bool {
	if function == nil || function.Pkg() == nil || function.Pkg().Path() != njectPackagePath {
		return false
	}
	for _, name := range names {
		if function.Name() == name {
			return true
		}
	}
	return false
}

func unindexedExpression(expression ast.Expr) ast.Expr {
	switch typed := expression.(type) {
	case *ast.IndexExpr:
		return unindexedExpression(typed.X)
	case *ast.IndexListExpr:
		return unindexedExpression(typed.X)
	default:
		return expression
	}
}
