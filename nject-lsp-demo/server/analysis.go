package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"unicode/utf16"
	"unicode/utf8"
)

type functionInfo struct {
	Name      string
	Filename  string
	Line      uint32
	Character uint32
}

// findFunction returns the selected consumer and the functions that
// transitively produce the selected parameter type in an nject.Run chain.
func findFunction(
	ctx context.Context,
	filename string,
	overlays map[string][]byte,
	line uint32,
	character uint32,
) ([]functionInfo, error) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("make file path absolute: %w", err)
	}

	normalized := make(map[string][]byte, len(overlays))
	for path, content := range overlays {
		absolute, pathErr := filepath.Abs(path)
		if pathErr != nil {
			continue
		}
		normalized[absolute] = content
	}

	workspace, err := loadAnalysisWorkspace(ctx, filename, normalized)
	if err != nil {
		return nil, err
	}
	if workspace == nil {
		return nil, nil
	}
	return workspace.findChain(filename, line, character)
}

func positionInSignature(function *ast.FuncDecl, position token.Pos) bool {
	start := function.Type.Pos()
	end := function.Type.End()
	if function.Body != nil {
		end = function.Body.Lbrace
	}
	return position >= start && position <= end
}

func positionInField(field *ast.Field, position token.Pos) bool {
	return position >= field.Pos() && position <= field.End()
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
