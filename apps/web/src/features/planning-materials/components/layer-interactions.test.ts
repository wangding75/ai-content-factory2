import assert from "node:assert/strict";
import test from "node:test";
import {createLayerStack, nextLayerFocusIndex} from "./layer-interactions.ts";

test("only the top material layer owns Escape handling", () => {
  const stack = createLayerStack();
  stack.add({id: 1, onEscape() {}});
  stack.add({id: 2, onEscape() {}});
  assert.equal(stack.isTop(1), false);
  assert.equal(stack.isTop(2), true);
  stack.remove(2);
  assert.equal(stack.isTop(1), true);
});

test("layer removal keeps the remaining stack intact", () => {
  const stack = createLayerStack();
  stack.add({id: 1, onEscape() {}});
  stack.add({id: 2, onEscape() {}});
  stack.remove(1);
  assert.equal(stack.size(), 1);
  assert.equal(stack.isTop(2), true);
});


test("Tab and Shift+Tab cycle inside the active layer", () => {
  assert.equal(nextLayerFocusIndex(3, 2, false), 0);
  assert.equal(nextLayerFocusIndex(3, 0, true), 2);
  assert.equal(nextLayerFocusIndex(3, 1, false), -1);
  assert.equal(nextLayerFocusIndex(3, -1, true), 2);
});