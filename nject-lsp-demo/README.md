# nject LSP Demo

This folder is a standalone VS Code extension with a Go Language Server
Protocol (LSP) server. It does not use or modify the nject library.

The server loads the selected file with `golang.org/x/tools/go/packages`,
including an overlay for unsaved editor content. It finds the declaration in
the Go AST and obtains its semantic `*types.Func` from `types.Info.Defs`.
Function names therefore come from Go's package loader and type checker, not
from a regular expression.

## Try it immediately

1. Open **this folder** (`nject-lsp-demo`) in VS Code.
2. Press **F5** and choose **Run nject LSP Demo** if prompted.
3. A new Extension Development Host opens with `demo/demo.go`.
4. Right-click anywhere on the `HelloNject` function declaration line.
5. Select **Refactor...**, then **nject LSP demo**.

The **nject LSP Demo** icon opens in the Activity Bar and its **Selected
Functions** view shows `HelloNject`. Click the item to jump back to the
function.

The action also works for Go methods, such as `Greeter.Greet`, and on unsaved
changes. It is offered while the cursor is in a typed function or method
signature.

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
- `server/analysis.go` performs package loading, AST lookup, and type lookup.
- `src/extension.ts` starts the Go server and owns the Activity Bar view.
- `demo/demo.go` is a ready-to-use sample.
