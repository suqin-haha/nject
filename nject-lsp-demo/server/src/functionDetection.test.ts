import assert from "node:assert/strict";
import test from "node:test";
import { findGoFunctionOnLine } from "./functionDetection";

test("finds a Go function declaration", () => {
  assert.deepEqual(findGoFunctionOnLine("func ProvideName() string {", 3), {
    name: "ProvideName",
    line: 3,
    character: 5,
  });
});

test("finds a Go method declaration", () => {
  assert.deepEqual(
    findGoFunctionOnLine("func (s *Server) Handle(value string) error {", 7),
    {
      name: "Handle",
      line: 7,
      character: 17,
    },
  );
});

test("ignores calls and non-function lines", () => {
  assert.equal(findGoFunctionOnLine("result := ProvideName()", 0), undefined);
});
