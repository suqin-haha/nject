# nject LSP Demo

This folder is a standalone VS Code extension with a Go Language Server
Protocol (LSP) server. It does not use or modify the nject library.

The server uses `golang.org/x/tools/go/packages`, ASTs, and `go/types`. It
rejects packages whose transitive dependency graph does not contain nject,
traces provider function values through helper parameters into
`nject.Run`/`MustRun`, and walks backward through exact Go input/output types.
Open-document overlays make unsaved changes visible to the analysis.

## Try it immediately

1. Open **this folder** (`nject-lsp-demo`) in VS Code.
2. Press **F5** and choose **Run nject LSP Demo** if prompted.
3. A new Extension Development Host opens with `demo/demo.go`.
4. Right-click the `name string` parameter of `HelloNject`.
5. Select **Nject: Find all in the Chain**.

The **nject LSP Demo** icon opens in the Activity Bar. Its **Selected
Functions** view shows `ProvideName`, which produces `string`, followed by the
selected consumer `HelloNject`. Click an item to jump to that function.

The initial implementation handles named functions, function literals,
variadic helper forwarding, variables, `Sequence`, `Cluster`, `Append`,
`Provide`, and common provider decorators. Runtime-generated reflective
providers remain opaque rather than producing speculative results.

The F5 build requires Node.js, npm, and Go. The Go command automatically
downloads the toolchain requested by `server/go.mod` when standard Go
toolchain auto-selection is enabled.

## Commands

```sh
npm install
npm test
npm run package
```

`npm run package` creates `nject-lsp-demo.vsix`, which can be installed with
VS Code's **Extensions: Install from VSIX...** command.

## Structure

- `server/main.go` implements the LSP transport and document lifecycle.
- `server/workspace.go` loads dependency-gated workspace packages and indexes
  typed declarations, call sites, parameters, and variables.
- `server/chain.go` resolves nject providers and traces producer types.
- `server/analysis.go` coordinates analysis and LSP position conversion.
- `src/extension.ts` starts the Go server and owns the Activity Bar view.
- `demo/demo.go` is a ready-to-use sample.
