import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("./pick-material-page.tsx", import.meta.url), "utf8");

test("pick material renders the dedicated four-region modal shell", () => {
  for (const region of ["pick-material-modal__header", "pick-material-modal__notice", "pick-material-modal__body", "pick-material-modal__footer"]) {
    assert.match(source, new RegExp(`className=\\"${region}`));
  }
  assert.match(source, /选择已有素材/);
  assert.match(source, /aria-label="关闭"/);
  assert.match(source, /绑定到项目/);
  assert.match(source, /取消/);
});

test("pick material keeps a single-column selectable list and a disabled unselected submit", () => {
  assert.match(source, /pick-material-modal__item/);
  assert.doesNotMatch(source, /pick-cards/);
  assert.match(source, /disabled=\{!selected\}/);
  assert.match(source, /closeMaterialLayer\("pick", projectId\)/);
});
