package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"
)

type functionInfo struct {
	Name      string
	Line      uint32
	Character uint32
}

// findFunction uses the same package-loading and type-checking machinery as
// Go developer tools. The overlay makes unsaved editor content visible to the
// type checker without writing it to disk.
func findFunction(
	ctx context.Context,
	filename string,
	content []byte,
	line uint32,
	character uint32,
) (*functionInfo, error) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("make file path absolute: %w", err)
	}

	cfg := &packages.Config{
		Context: ctx,
		Dir:     filepath.Dir(filename),
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes,
		Overlay: map[string][]byte{filename: content},
	}
	loaded, err := packages.Load(cfg, "file="+filename)
	if err != nil {
		return nil, fmt.Errorf("load package: %w", err)
	}

	offset, ok := offsetForPosition(content, line, character)
	if !ok {
		return nil, nil
	}

	for _, pkg := range loaded {
		for index, syntax := range pkg.Syntax {
			if index >= len(pkg.CompiledGoFiles) {
				continue
			}
			compiledFile, err := filepath.Abs(pkg.CompiledGoFiles[index])
			if err != nil || compiledFile != filename {
				continue
			}

			tokenFile := pkg.Fset.File(syntax.Pos())
			if tokenFile == nil || offset > tokenFile.Size() {
				return nil, nil
			}
			position := tokenFile.Pos(offset)

			for _, declaration := range syntax.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || !positionInSignature(function, position) {
					continue
				}

				object, ok := pkg.TypesInfo.Defs[function.Name].(*types.Func)
				if !ok {
					return nil, nil
				}
				nameOffset := tokenFile.Offset(function.Name.Pos())
				nameLine, nameCharacter, ok := positionForOffset(content, nameOffset)
				if !ok {
					return nil, nil
				}
				return &functionInfo{
					Name:      object.Name(),
					Line:      nameLine,
					Character: nameCharacter,
				}, nil
			}
		}
	}
	return nil, nil
}

func positionInSignature(function *ast.FuncDecl, position token.Pos) bool {
	start := function.Type.Pos()
	end := function.Type.End()
	if function.Body != nil {
		end = function.Body.Lbrace
	}
	return position >= start && position <= end
}

// offsetForPosition converts an LSP UTF-16 position to a Go byte offset.
func offsetForPosition(content []byte, targetLine uint32, targetCharacter uint32) (int, bool) {
	line := uint32(0)
	offset := 0
	for line < targetLine {
		index := offset
		for index < len(content) && content[index] != '\n' {
			index++
		}
		if index == len(content) {
			return 0, false
		}
		offset = index + 1
		line++
	}

	units := uint32(0)
	for offset < len(content) && content[offset] != '\n' {
		r, size := utf8.DecodeRune(content[offset:])
		width := uint32(len(utf16.Encode([]rune{r})))
		if units+width > targetCharacter {
			return offset, true
		}
		if units+width == targetCharacter {
			return offset + size, true
		}
		units += width
		offset += size
	}
	return offset, units == targetCharacter
}

func positionForOffset(content []byte, target int) (uint32, uint32, bool) {
	if target < 0 || target > len(content) {
		return 0, 0, false
	}
	line := uint32(0)
	character := uint32(0)
	for offset := 0; offset < target; {
		r, size := utf8.DecodeRune(content[offset:])
		if offset+size > target {
			return 0, 0, false
		}
		if r == '\n' {
			line++
			character = 0
		} else {
			character += uint32(len(utf16.Encode([]rune{r})))
		}
		offset += size
	}
	return line, character, true
}
