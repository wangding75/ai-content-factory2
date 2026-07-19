import assert from "node:assert/strict";
import test from "node:test";
import { updatePayload, validate, type Form, type PlatformType } from "./distribution-api.ts";

const type:PlatformType={platformType:"custom",displayName:"自定义平台",authTypes:["api_key"],fieldSchemas:[]};
const form:Form={name:"p",platformType:"custom",accountIdentifier:"a",endpointUrl:"https://example.com",authType:"api_key",timeoutSeconds:30,typeConfig:{},note:"",credential:""};
test("validates platform contract fields and excludes readonly platform type on update",()=>{assert.match(validate({...form,endpointUrl:""},true,type)??"",/连接地址/);assert.match(validate({...form,timeoutSeconds:301},true,type)??"",/5 至 300/);assert.equal("platformType" in updatePayload(form,1),false);});
