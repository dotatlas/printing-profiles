import { expect, test } from "vitest";
import { handleProblem, toHandle } from "./welcome";

test("toHandle shapes typed text into a handle as the visitor types", () => {
  expect(toHandle("Alice Prints")).toBe("alice-prints");
  expect(toHandle("my_PLA__tuner")).toBe("my-pla-tuner");
  expect(toHandle("émile!!")).toBe("emile");
  expect(toHandle("Aleksandra Wiśniewska")).toBe("aleksandra-wisniewska");
  expect(toHandle("a".repeat(40))).toHaveLength(30);
});

test("handleProblem matches the API's handle rule", () => {
  expect(handleProblem("ab")).not.toBe("");
  expect(handleProblem("-ab")).not.toBe("");
  expect(handleProblem("ab-")).not.toBe("");
  expect(handleProblem("abc")).toBe("");
  expect(handleProblem("pla-tuner-2")).toBe("");
});
