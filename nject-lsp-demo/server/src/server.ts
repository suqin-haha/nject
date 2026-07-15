import {
  CodeAction,
  CodeActionKind,
  CodeActionParams,
  createConnection,
  InitializeParams,
  InitializeResult,
  ProposedFeatures,
  TextDocumentSyncKind,
} from "vscode-languageserver/node";
import { TextDocuments } from "vscode-languageserver";
import { TextDocument } from "vscode-languageserver-textdocument";
import { findGoFunctionOnLine } from "./functionDetection";

const connection = createConnection(ProposedFeatures.all);
const documents = new TextDocuments(TextDocument);

connection.onInitialize((_params: InitializeParams): InitializeResult => ({
  capabilities: {
    textDocumentSync: TextDocumentSyncKind.Incremental,
    codeActionProvider: {
      codeActionKinds: [CodeActionKind.Refactor],
    },
  },
}));

connection.onCodeAction((params: CodeActionParams): CodeAction[] => {
  const document = documents.get(params.textDocument.uri);
  if (!document) {
    return [];
  }

  const line = params.range.start.line;
  const lineStart = { line, character: 0 };
  const lineEnd = { line: line + 1, character: 0 };
  const functionMatch = findGoFunctionOnLine(
    document.getText({ start: lineStart, end: lineEnd }),
    line,
  );
  if (!functionMatch) {
    return [];
  }

  return [
    {
      title: "nject LSP demo",
      kind: CodeActionKind.Refactor,
      command: {
        title: "nject LSP demo",
        command: "njectLspDemo.showFunction",
        arguments: [
          {
            name: functionMatch.name,
            uri: params.textDocument.uri,
            line: functionMatch.line,
            character: functionMatch.character,
          },
        ],
      },
    },
  ];
});

documents.listen(connection);
connection.listen();
