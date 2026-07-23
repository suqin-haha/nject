package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFindFunctionRejectsPackageWithoutNjectDependency(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module example.com/demo\n\ngo 1.22\n")
	filename := filepath.Join(directory, "demo.go")
	content := []byte(`package demo

type Input struct{}

func Consume(input Input) {}
`)
	writeTestFile(t, filename, string(content))
	line, character := testPosition(content, "input Input", len("input "))
	functions, err := findFunction(
		context.Background(),
		filename,
		map[string][]byte{filename: content},
		line,
		character,
	)
	if err != nil {
		t.Fatal(err)
	}
	if functions != nil {
		t.Fatalf("expected no result, got %+v", functions)
	}
}

func TestFindFunctionTracesCrossPackageNjectProviderChain(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(
		t,
		filepath.Join(directory, "go.mod"),
		"module github.com/muir/nject/v2\n\ngo 1.22\n",
	)
	writeTestFile(t, filepath.Join(directory, "nject.go"), `package nject

type Provider interface{}

func Run(name string, providers ...any) {}
func MustRun(name string, providers ...any) {}
func Provide(name string, provider any) Provider { return provider }
func Required(provider any) Provider { return provider }
`)
	writeTestFile(t, filepath.Join(directory, "runner", "runner.go"), `package runner

import nject "github.com/muir/nject/v2"

func RunProviders(providers ...any) {
	nject.Run("demo", providers...)
}
`)
	filename := filepath.Join(directory, "app", "app.go")
	content := []byte(`package app

import (
	nject "github.com/muir/nject/v2"
	"github.com/muir/nject/v2/runner"
)

type Config struct{}
type Input struct{}

func MakeConfig() Config { return Config{} }
func MakeInput(config Config) Input { return Input{} }
func Consume(input Input) {}

func Start() {
	runner.RunProviders(
		MakeConfig,
		nject.Required(nject.Provide("input", MakeInput)),
		Consume,
	)
}
`)
	writeTestFile(t, filename, string(content))
	line, character := testPosition(content, "input Input", len("input "))
	functions, err := findFunction(
		context.Background(),
		filename,
		map[string][]byte{filename: content},
		line,
		character,
	)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, function := range functions {
		names = append(names, function.Name)
	}
	slices.Sort(names)
	if want := []string{"Consume", "MakeConfig", "MakeInput"}; !slices.Equal(names, want) {
		t.Fatalf("got functions %v, want %v", names, want)
	}
}

func TestFindFunctionRequiresCursorOnParameter(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(
		t,
		filepath.Join(directory, "go.mod"),
		"module github.com/muir/nject/v2\n\ngo 1.22\n",
	)
	filename := filepath.Join(directory, "demo.go")
	content := []byte(`package nject

func Run(name string, providers ...any) {}
func Consume(input string) {}

func Start() {
	Run("demo", Consume)
}
`)
	writeTestFile(t, filename, string(content))
	line, character := testPosition(content, "Run(\"demo\"", 1)
	functions, err := findFunction(
		context.Background(),
		filename,
		map[string][]byte{filename: content},
		line,
		character,
	)
	if err != nil {
		t.Fatal(err)
	}
	if functions != nil {
		t.Fatalf("expected no result, got %+v", functions)
	}
}

func writeTestFile(t *testing.T, filename string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func testPosition(content []byte, marker string, markerOffset int) (uint32, uint32) {
	offset := strings.Index(string(content), marker) + markerOffset
	prefix := string(content[:offset])
	line := uint32(strings.Count(prefix, "\n"))
	lastNewline := strings.LastIndex(prefix, "\n")
	return line, uint32(len(prefix) - lastNewline - 1)
}
