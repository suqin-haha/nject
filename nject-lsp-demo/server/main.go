package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

const (
	serverName    = "nject LSP Demo"
	serverVersion = "0.2.0"
)

type documentStore struct {
	mu      sync.RWMutex
	content map[string][]byte
}

func newDocumentStore() *documentStore {
	return &documentStore{content: make(map[string][]byte)}
}

func (s *documentStore) put(uri string, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.content[uri] = []byte(content)
}

func (s *documentStore) get(uri string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	content, ok := s.content[uri]
	return append([]byte(nil), content...), ok
}

func (s *documentStore) remove(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.content, uri)
}

func (s *documentStore) overlays() map[string][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	overlays := make(map[string][]byte, len(s.content))
	for uri, content := range s.content {
		filename, err := filePath(uri)
		if err != nil {
			continue
		}
		overlays[filename] = append([]byte(nil), content...)
	}
	return overlays
}

type languageServer struct {
	documents *documentStore
	handler   protocol.Handler
}

func newLanguageServer() *languageServer {
	lsp := &languageServer{documents: newDocumentStore()}
	lsp.handler = protocol.Handler{
		Initialize:             lsp.initialize,
		Shutdown:               lsp.shutdown,
		TextDocumentDidOpen:    lsp.didOpen,
		TextDocumentDidChange:  lsp.didChange,
		TextDocumentDidClose:   lsp.didClose,
		TextDocumentCodeAction: lsp.codeAction,
	}
	return lsp
}

func (s *languageServer) initialize(
	_ *glsp.Context,
	_ *protocol.InitializeParams,
) (any, error) {
	capabilities := s.handler.CreateServerCapabilities()
	full := protocol.TextDocumentSyncKindFull
	capabilities.TextDocumentSync.(*protocol.TextDocumentSyncOptions).Change = &full
	capabilities.CodeActionProvider = &protocol.CodeActionOptions{
		CodeActionKinds: []protocol.CodeActionKind{protocol.CodeActionKindRefactor},
	}
	version := serverVersion
	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    serverName,
			Version: &version,
		},
	}, nil
}

func (s *languageServer) shutdown(_ *glsp.Context) error {
	return nil
}

func (s *languageServer) didOpen(
	_ *glsp.Context,
	params *protocol.DidOpenTextDocumentParams,
) error {
	s.documents.put(params.TextDocument.URI, params.TextDocument.Text)
	return nil
}

func (s *languageServer) didChange(
	_ *glsp.Context,
	params *protocol.DidChangeTextDocumentParams,
) error {
	if len(params.ContentChanges) == 0 {
		return nil
	}
	switch change := params.ContentChanges[len(params.ContentChanges)-1].(type) {
	case protocol.TextDocumentContentChangeEventWhole:
		s.documents.put(params.TextDocument.URI, change.Text)
	case *protocol.TextDocumentContentChangeEventWhole:
		s.documents.put(params.TextDocument.URI, change.Text)
	default:
		return fmt.Errorf("expected full document synchronization")
	}
	return nil
}

func (s *languageServer) didClose(
	_ *glsp.Context,
	params *protocol.DidCloseTextDocumentParams,
) error {
	s.documents.remove(params.TextDocument.URI)
	return nil
}

func (s *languageServer) codeAction(
	_ *glsp.Context,
	params *protocol.CodeActionParams,
) (any, error) {
	filename, err := filePath(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	content, ok := s.documents.get(params.TextDocument.URI)
	if !ok {
		content, err = os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read document: %w", err)
		}
	}

	overlays := s.documents.overlays()
	overlays[filename] = content
	functions, err := findFunction(
		context.Background(),
		filename,
		overlays,
		params.Range.Start.Line,
		params.Range.Start.Character,
	)
	if err != nil {
		return nil, err
	}
	if len(functions) == 0 {
		return []protocol.CodeAction{}, nil
	}

	items := make([]map[string]any, 0, len(functions))
	for _, function := range functions {
		items = append(items, map[string]any{
			"name":      function.Name,
			"uri":       pathURI(function.Filename),
			"line":      function.Line,
			"character": function.Character,
		})
	}

	kind := protocol.CodeActionKindRefactor
	return []protocol.CodeAction{{
		Title: "Nject: Find all in the Chain",
		Kind:  &kind,
		Command: &protocol.Command{
			Title:   "Nject: Find all in the Chain",
			Command: "njectLspDemo.showFunction",
			Arguments: []any{map[string]any{
				"functions": items,
			}},
		},
	}}, nil
}

func filePath(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("parse document URI: %w", err)
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("unsupported document URI scheme %q", parsed.Scheme)
	}
	return filepath.FromSlash(parsed.Path), nil
}

func pathURI(filename string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(filename)}).String()
}

func main() {
	lsp := newLanguageServer()
	if err := server.NewServer(&lsp.handler, serverName, false).RunStdio(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
