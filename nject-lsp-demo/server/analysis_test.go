package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFindFunctionUsesTypedPackage(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(directory, "go.mod"),
		[]byte("module example.com/demo\n\ngo 1.22\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(directory, "demo.go")
	onDisk := []byte("package demo\n\nfunc OldName() {}\n")
	if err := os.WriteFile(filename, onDisk, 0o600); err != nil {
		t.Fatal(err)
	}

	// The overlay deliberately differs from disk to verify that unsaved editor
	// content is what go/packages parses and go/types resolves.
	overlay := []byte(`package demo

type Greeter struct{}

func (Greeter) HelloNject(name string) string {
	return name
}
`)
	function, err := findFunction(context.Background(), filename, overlay, 4, 31)
	if err != nil {
		t.Fatal(err)
	}
	if function == nil {
		t.Fatal("expected a typed function")
	}
	if function.Name != "HelloNject" {
		t.Fatalf("got function %q, want HelloNject", function.Name)
	}
	if function.Line != 4 || function.Character != 15 {
		t.Fatalf("got position %d:%d, want 4:15", function.Line, function.Character)
	}
}

func TestFindFunctionIgnoresFunctionBody(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(directory, "go.mod"),
		[]byte("module example.com/demo\n\ngo 1.22\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(directory, "demo.go")
	content := []byte(`package demo

func HelloNject() {
	println("body")
}
`)
	if err := os.WriteFile(filename, content, 0o600); err != nil {
		t.Fatal(err)
	}

	function, err := findFunction(context.Background(), filename, content, 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if function != nil {
		t.Fatalf("unexpected function in body: %s", function.Name)
	}
}
