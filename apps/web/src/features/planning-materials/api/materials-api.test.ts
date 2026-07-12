import assert from "node:assert/strict";
import test from "node:test";
import { listProjectMaterials } from "./materials-api.ts";
import { planningFixtureProject } from "../mock/fixtures.ts";
const id=planningFixtureProject.id;
test("returns an empty project material list",async()=>{const r=await listProjectMaterials(id,{mockScenario:"empty"});assert.deepEqual(r.items,[]);assert.deepEqual(r.type_counts,{character:0,worldview:0,location:0,organization:0,item:0,reference:0})});
test("applies q and type with AND semantics",async()=>{const r=await listProjectMaterials(id,{q:"主角",type:"character"});assert.equal(r.total,1);assert.equal(r.items[0].material.name,"林野")});
test("type counts ignore filters",async()=>{const r=await listProjectMaterials(id,{type:"character",q:"林野"});assert.equal(r.type_counts.character,2);assert.equal(r.type_counts.reference,1)});
test("uses material id as stable tiebreaker",async()=>{const r=await listProjectMaterials(id,{sort:"name_asc"});assert.ok(r.items.every((x,i,a)=>i===0||a[i-1].material.name.localeCompare(x.material.name,"zh-CN")<=0))});
