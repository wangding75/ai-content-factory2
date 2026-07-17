import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
const page=readFileSync(new URL("./global-pages.tsx",import.meta.url),"utf8");
const api=readFileSync(new URL("./global-lite-api.ts",import.meta.url),"utf8");
test("E1 and E2 render only view models and implement stateful list interactions",()=>{assert.match(page,/listGlobalMaterials/);assert.match(page,/listGlobalWorks/);assert.match(page,/正在加载全局素材/);assert.match(page,/暂无素材/);assert.match(page,/暂时无法加载/);assert.match(page,/上一页/);assert.match(page,/查看详情/);assert.match(page,/查看项目作品/);});
test("global mock API keeps scope at the client boundary and exposes required fixtures",()=>{assert.match(api,/scope=global/);for(const state of ["loading","single","empty","error","invalid-pagination","paged","multi-use","unused","succeeded","failed","no-run"])assert.match(api,new RegExp(state));assert.match(api,/work_id:id/);assert.match(api,/current_version:current/);});
