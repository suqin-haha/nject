export interface FunctionMatch {
  name: string;
  line: number;
  character: number;
}

const FUNCTION_DECLARATION =
  /\bfunc\s*(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*(?:\[[^\]]*\]\s*)?\(/;

/**
 * Finds a Go function or method declaration on a line. The action is offered
 * anywhere on the declaration line, so users can right-click `func`, the
 * function name, a parameter type, or a return type.
 */
export function findGoFunctionOnLine(
  lineText: string,
  line: number,
): FunctionMatch | undefined {
  const match = FUNCTION_DECLARATION.exec(lineText);
  if (!match || match.index === undefined) {
    return undefined;
  }

  const name = match[1];
  const relativeCharacter = match[0].indexOf(name);
  return {
    name,
    line,
    character: match.index + relativeCharacter,
  };
}
