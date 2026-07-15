import * as path from "node:path";
import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from "vscode-languageclient/node";

interface ShowFunctionArguments {
  name: string;
  uri: string;
  line: number;
  character: number;
}

class FunctionItem extends vscode.TreeItem {
  constructor(args: ShowFunctionArguments) {
    super(args.name, vscode.TreeItemCollapsibleState.None);
    this.description = vscode.Uri.parse(args.uri).fsPath;
    this.iconPath = new vscode.ThemeIcon("symbol-function");
    this.command = {
      command: "vscode.open",
      title: "Open function",
      arguments: [
        vscode.Uri.parse(args.uri),
        {
          selection: new vscode.Range(
            args.line,
            args.character,
            args.line,
            args.character,
          ),
        },
      ],
    };
  }
}

class FunctionProvider implements vscode.TreeDataProvider<FunctionItem> {
  private readonly changed = new vscode.EventEmitter<
    FunctionItem | undefined | null | void
  >();
  private selected?: FunctionItem;

  readonly onDidChangeTreeData = this.changed.event;

  select(args: ShowFunctionArguments): void {
    this.selected = new FunctionItem(args);
    this.changed.fire();
  }

  getTreeItem(element: FunctionItem): vscode.TreeItem {
    return element;
  }

  getChildren(): FunctionItem[] {
    return this.selected ? [this.selected] : [];
  }
}

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const provider = new FunctionProvider();
  context.subscriptions.push(
    vscode.window.registerTreeDataProvider("njectLspDemo.functions", provider),
    vscode.commands.registerCommand(
      "njectLspDemo.showFunction",
      async (args: ShowFunctionArguments) => {
        provider.select(args);
        await vscode.commands.executeCommand(
          "workbench.view.extension.njectLspDemo",
        );
      },
    ),
  );

  const serverExecutable = context.asAbsolutePath(
    path.join(
      "bin",
      process.platform === "win32" ? "nject-lsp.exe" : "nject-lsp",
    ),
  );
  const serverOptions: ServerOptions = {
    command: serverExecutable,
    args: [],
  };
  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "go" }],
  };

  client = new LanguageClient(
    "njectLspDemo",
    "nject LSP Demo",
    serverOptions,
    clientOptions,
  );
  await client.start();
}

export async function deactivate(): Promise<void> {
  if (client) {
    await client.stop();
  }
}
