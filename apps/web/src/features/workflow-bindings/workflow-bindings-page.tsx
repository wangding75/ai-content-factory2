"use client";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "@/lib/api";
import { bindWorkflow, bindingCopy, listApplicableWorkflows, listProjectWorkflowBindings, newIdempotencyKey, stageDescriptions, stageLabels, stageOrder, type BindingStage, type WorkflowConfiguration, unbindWorkflow } from "./workflow-binding-api";

type Drawer={stage:BindingStage; mode:"select"|"replace"}|null;
const safeError=(error:unknown)=>error instanceof ApiError&&error.code.toLowerCase()==="version_conflict"?"配置已在其他位置更新。请加载最新状态后重新确认。":"操作未完成，请稍后重试。";
export function WorkflowBindingsPage({projectId}:{projectId:string}){
 const [items,setItems]=useState<BindingStage[]|null>(null),[error,setError]=useState<string|null>(null),[drawer,setDrawer]=useState<Drawer>(null),[unbind,setUnbind]=useState<BindingStage|null>(null),[notice,setNotice]=useState<string|null>(null);
 const load=useCallback(async(signal?:AbortSignal)=>{setError(null);try{const data=await listProjectWorkflowBindings(projectId,{signal});if(!signal?.aborted)setItems(data.items)}catch{if(!signal?.aborted)setError("暂时无法加载工作流绑定，请稍后重试。")}},[projectId]);
 useEffect(()=>{const c=new AbortController();const timer=window.setTimeout(()=>void load(c.signal),0);return()=>{window.clearTimeout(timer);c.abort()}},[load]);
 const refresh=async()=>{await load();};
 const doUnbind=async()=>{if(!unbind?.binding)return;try{await unbindWorkflow(projectId,unbind.stage,unbind.binding.version,newIdempotencyKey());setUnbind(null);setNotice("已解除当前项目的工作流绑定。");await refresh()}catch(e){setNotice(safeError(e));}};
 if(error)return <section className="workflow-binding-state" role="alert"><h2>暂时无法加载工作流绑定</h2><p>{error}</p><button onClick={()=>void load()}>重试</button></section>;
 if(!items)return <section className="workflow-binding-skeleton" aria-label="正在加载工作流绑定">{stageOrder.map(x=><div key={x}/>)}</section>;
 const ordered=stageOrder.map(stage=>items.find(x=>x.stage===stage)??{stage,bound:false,binding:null,workflowConfigurationSummary:null});const completed=ordered.filter(x=>x.bound).length;
 return <section className="workflow-bindings"><header className="workflow-bindings-heading"><div><h2>工作流绑定</h2><p>为项目的四个环节分别选择可复用的工作流。绑定不代表该工作流已经验证或可以执行。</p></div><strong>{completed} / 4 已绑定</strong></header>{notice&&<p className="workflow-binding-notice" role="status">{notice}</p>}<div className="workflow-binding-grid">{ordered.map(item=><BindingCard key={item.stage} item={item} onSelect={()=>setDrawer({stage:item,mode:item.bound?"replace":"select"})} onUnbind={()=>setUnbind(item)}/>)}</div>{drawer&&<WorkflowDrawer projectId={projectId} drawer={drawer} onClose={()=>setDrawer(null)} onSaved={async()=>{setDrawer(null);setNotice("工作流绑定已更新。");await refresh();}}/>}{unbind&&<UnbindDialog item={unbind} onClose={()=>setUnbind(null)} onConfirm={doUnbind}/>}</section>
}
function BindingCard({item,onSelect,onUnbind}:{item:BindingStage;onSelect:()=>void;onUnbind:()=>void}){
  const workflow=item.workflowConfigurationSummary,copy=bindingCopy(item);
  let badgeClass = "muted";
  if (item.bound) {
    if (copy.exceptionType === "disabled") badgeClass = "badge-disabled";
    else if (copy.exceptionType === "integration_error") badgeClass = "badge-warning";
    else if (copy.exceptionType === "connection_error") badgeClass = "badge-danger";
    else badgeClass = "good";
  }

  return (
    <article className="workflow-binding-card">
      <header>
        <div>
          <h3>{stageLabels[item.stage]}</h3>
          <p>{stageDescriptions[item.stage]}</p>
        </div>
        <span className={badgeClass}>{copy.statusText}</span>
      </header>
      {workflow ? (
        <div className="workflow-binding-details">
          <strong title={workflow.name}>{workflow.name}</strong>
          <dl>
            <div><dt>类型</dt><dd>{workflow.workflowType}</dd></div>
            <div><dt>连接</dt><dd>{copy.connection}（{workflow.connectionType}）</dd></div>
            <div><dt>启用状态</dt><dd>{copy.enabled}</dd></div>
            <div><dt>集成状态</dt><dd>{copy.integration}</dd></div>
          </dl>
          {copy.exceptionType === "disabled" && (
            <p className="workflow-warning warning-disabled">当前工作流在全局设置中已被管理员停用，无法在该项目中执行。</p>
          )}
          {copy.exceptionType === "integration_error" && (
            <p className="workflow-warning warning-integration">工作流配置存在但尚未完成系统接入，当前暂不可用，请等待集成完成。</p>
          )}
          {copy.exceptionType === "connection_error" && (
            <p className="workflow-warning warning-connection">无法连接到目标服务环境。请检查网络状态或服务是否正常运行。</p>
          )}
        </div>
      ) : (
        <div className="workflow-binding-empty">
          <p>尚未绑定工作流</p>
          <small>选择适用于此环节的工作流后，项目将保存独立的绑定关系。</small>
        </div>
      )}
      <footer>
        {item.bound ? (
          <>
            <button onClick={onSelect}>更换工作流</button>
            <button className="danger" onClick={onUnbind}>解除绑定</button>
          </>
        ) : (
          <button className="primary" onClick={onSelect}>选择工作流</button>
        )}
      </footer>
    </article>
  );
}

function WorkflowDrawer({projectId,drawer,onClose,onSaved}:{projectId:string;drawer:Exclude<Drawer,null>;onClose:()=>void;onSaved:()=>Promise<void>}){
  const [query,setQuery]=useState(""),[workflows,setWorkflows]=useState<WorkflowConfiguration[]|null>(null),[error,setError]=useState(false),[selected,setSelected]=useState(drawer.stage.workflowConfigurationSummary?.id??""),[saving,setSaving]=useState(false),[conflict,setConflict]=useState(false);
  const key=useRef(newIdempotencyKey());

  const load=useCallback(async(signal?:AbortSignal)=>{
    setError(false);
    try{
      const data=await listApplicableWorkflows(drawer.stage.stage,query,{signal});
      if(!signal?.aborted)setWorkflows(data.items);
    }catch{
      if(!signal?.aborted)setError(true);
    }
  },[drawer.stage.stage,query]);

  useEffect(()=>{
    const c=new AbortController();
    const t=window.setTimeout(()=>void load(c.signal),250);
    return()=>{clearTimeout(t);c.abort()};
  },[load]);

  useEffect(()=>{
    const esc=(e:KeyboardEvent)=>e.key==="Escape"&&!saving&&onClose();
    window.addEventListener("keydown",esc);
    return()=>window.removeEventListener("keydown",esc);
  },[onClose,saving]);

  const currentId = drawer.stage.workflowConfigurationSummary?.id ?? "";
  const isReplace = drawer.mode === "replace";
  const submitDisabled = !selected || saving || (isReplace && selected === currentId);

  const submit=async()=>{
    if(submitDisabled)return;
    setSaving(true);
    setConflict(false);
    try{
      await bindWorkflow(projectId,drawer.stage.stage,selected,drawer.stage.binding?.version,key.current);
      await onSaved();
    }catch(e){
      if(e instanceof ApiError&&e.code.toLowerCase()==="version_conflict")setConflict(true);
      else setError(true);
    }finally{
      setSaving(false);
    }
  };

  return (
    <div className="workflow-drawer-layer" role="presentation">
      <button className="workflow-drawer-backdrop" aria-label="关闭选择工作流" onClick={onClose} disabled={saving}/>
      <aside className="workflow-binding-drawer" role="dialog" aria-modal="true" aria-labelledby="workflow-drawer-title">
        <header>
          <div>
            <h2 id="workflow-drawer-title">{isReplace ? "更换工作流" : "选择工作流"}</h2>
            <p>{stageLabels[drawer.stage.stage]} · 仅显示适用于此环节的工作流</p>
          </div>
          <button onClick={onClose} disabled={saving} aria-label="关闭">×</button>
        </header>
        <div className="workflow-drawer-body">
          <label>搜索工作流
            <input autoFocus value={query} onChange={e=>setQuery(e.target.value)} placeholder="输入工作流名称"/>
          </label>
          {error ? (
            <div className="workflow-binding-state" role="alert">
              <p>候选工作流加载失败。</p>
              <button onClick={()=>void load()}>重试</button>
            </div>
          ) : !workflows ? (
            <p>正在加载候选工作流…</p>
          ) : workflows.length === 0 ? (
            <div className="workflow-binding-state">
              <h3>没有可用的工作流</h3>
              <p>请先在全局设置中创建并配置适用于“{stageLabels[drawer.stage.stage]}”的工作流。</p>
              <Link href={`/workflows`}>前往全局设置</Link>
            </div>
          ) : (
            <div className="workflow-candidates">
              {workflows.map(w => {
                const disabled = !w.enabled;
                const isCurrent = currentId === w.id;
                let candidateHint = "已集成";
                if (disabled) candidateHint = "已停用，不能选择";
                else if (w.integrationStatus === "not_connected") candidateHint = "未接入：可绑定，但需注意配置风险";
                else if (w.integrationStatus === "connection_error" || Boolean(w.lastErrorMessage) || w.connectionName === "连接异常") candidateHint = "连接异常：可绑定，但需检查服务环境";

                return (
                  <label
                    key={w.id}
                    className={`workflow-candidate ${selected === w.id ? "selected" : ""} ${disabled ? "disabled" : ""}`}
                    onClick={(e) => { if (disabled) e.preventDefault(); }}
                  >
                    <input
                      type="radio"
                      name="workflow"
                      value={w.id}
                      checked={selected === w.id}
                      disabled={disabled}
                      onChange={() => { if (!disabled) setSelected(w.id); }}
                    />
                    <span>
                      <strong title={w.name}>{w.name}</strong>
                      <small>{w.workflowType} · {w.connectionName}（{w.connectionType}）</small>
                      <em>{candidateHint}</em>
                    </span>
                    {isCurrent && <b>当前绑定</b>}
                  </label>
                );
              })}
            </div>
          )}
          {conflict && (
            <div className="workflow-conflict" role="alert">
              <p>当前绑定已更新，未覆盖服务端数据。你的选择仍被保留。</p>
              <button onClick={async()=>{await load();setConflict(false)}}>加载最新状态后重新确认</button>
            </div>
          )}
        </div>
        <footer>
          <button onClick={onClose} disabled={saving}>取消</button>
          <button className="primary" disabled={submitDisabled} onClick={submit}>
            {saving ? "保存中…" : isReplace ? "保存更换" : "确认绑定"}
          </button>
        </footer>
      </aside>
    </div>
  );
}

function UnbindDialog({item,onClose,onConfirm}:{item:BindingStage;onClose:()=>void;onConfirm:()=>void}){
  const name=item.workflowConfigurationSummary?.name??"当前工作流";
  return (
    <div className="workflow-dialog-layer" role="presentation">
      <button className="workflow-drawer-backdrop" aria-label="取消解除绑定" onClick={onClose}/>
      <section className="workflow-unbind-dialog" role="dialog" aria-modal="true" aria-labelledby="unbind-title">
        <h2 id="unbind-title">确认解除绑定？</h2>
        <p>将解除“{stageLabels[item.stage]}”与“{name}”的项目关系。</p>
        <p>此操作不会删除全局工作流，也不会影响其他项目。</p>
        <footer>
          <button onClick={onClose}>取消</button>
          <button className="danger" onClick={onConfirm}>确认解除绑定</button>
        </footer>
      </section>
    </div>
  );
}
