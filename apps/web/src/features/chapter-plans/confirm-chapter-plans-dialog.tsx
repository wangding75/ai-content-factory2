"use client";

import {Icon} from "@/components/ui/icons";
import {ApiError} from "@/lib/api";
import type {ChapterPlan} from "./chapter-plan-http-api";

export function ConfirmChapterPlansDialog({plans,onClose,onConfirm,submitting,error}:{plans:ChapterPlan[];onClose:()=>void;onConfirm:()=>void;submitting:boolean;error:ApiError|null}){
  const chapters=plans.map(plan=>plan.chapter_no).sort((a,b)=>a-b);
  const chapterRange=chapters.length===1?`第 ${chapters[0]} 章`:`第 ${chapters[0]}–${chapters.at(-1)} 章`;
  const message=error?.status===409?"确认数据已发生变化。请刷新列表后重新选择并提交。":error?"暂时无法确认章节规划，请检查网络后重试。":null;
  return <div className="chapter-confirm-backdrop" role="presentation"><section className="chapter-confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="chapter-confirm-title"><header><div className="chapter-confirm-heading"><div className="chapter-confirm-icon"><Icon name="timeline" size={24}/></div><div><h2 id="chapter-confirm-title">确认章节规划</h2><p>确认后，所选候选章节将进入可生产状态。</p></div></div><button type="button" aria-label="关闭确认章节规划" onClick={onClose} disabled={submitting}>×</button></header><div className="chapter-confirm-body"><p className="chapter-confirm-summary"><b>已选择：{plans.length} 个候选章节</b><i>|</i><span>章节范围：{chapterRange}</span><i>|</i><span>来源：模拟生成</span></p><section className="chapter-confirm-selected" aria-label="已选择章节">{plans.map(plan=><article key={plan.id}><div><b>{plan.chapter_no}</b><strong>{plan.title}</strong></div><p>{plan.summary||"暂无章节摘要"}</p></article>)}</section><section className="chapter-confirm-validation"><h3>系统校验</h3>{["章节序号无重复","章节均位于故事线范围内","每章已设置主故事线","关联伏笔章节范围有效","未覆盖现有已确认章节"].map(text=><p key={text}><Icon name="timeline" size={16}/>{text}</p>)}</section><section className="chapter-confirm-after"><h3><Icon name="info" size={16}/>确认后：</h3><ul><li>状态转为“已确认”</li><li>可进入正文生产</li><li>来源记录保留</li><li>不会自动生成正文</li></ul></section>{message&&<p className="chapter-confirm-error" role="alert">{message}</p>}</div><footer><button type="button" onClick={onClose} disabled={submitting}>返回检查</button><button type="button" onClick={onConfirm} disabled={submitting||plans.length===0}>{submitting?"确认中…":"确认章节规划"}</button></footer></section></div>;
}
