# nject LSP Demo

This folder is a standalone VS Code extension and Language Server Protocol
(LSP) server. It does not use or modify the nject library.

## Try it immediately

1. Open **this folder** (`nject-lsp-demo`) in VS Code.
2. Press **F5** and choose **Run nject LSP Demo** if prompted.
3. A new Extension Development Host opens with `demo/demo.go`.
4. Right-click anywhere on the `HelloNject` function declaration line.
5. Select **Refactor...**, then **nject LSP demo**.

The **nject LSP Demo** icon opens in the Activity Bar and its **Selected
Functions** view shows `HelloNject`. Click the item to jump back to the
function.

The action also works for Go methods, such as `Greeter.Greet`. It is only
offered on a line containing a Go function or method declaration.

## Commands

```sh
npm install
npm test
npm run package
```

`npm run package` creates `nject-lsp-demo.vsix`, which can be installed with
VS Code's **Extensions: Install from VSIX...** command.

## Structure

- `server/src/server.ts` is the LSP server and supplies the code action.
- `src/extension.ts` starts the server and owns the Activity Bar view.
- `demo/demo.go` is a ready-to-use sample.
