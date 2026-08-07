"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ApiError } from "@/lib/api";
import { listBuiltinWorkflows, type BuiltinWorkflowVm } from "./global-lite-api";

function ErrorState({ error, retry }: { error: ApiError; retry: () => void }) {
  return (
    <section className="lite-state" role="alert">
      <h2>暂时无法加载</h2>
      <p>{error.message}</p>
      <button onClick={retry}>重试</button>
    </section>
  );
}

function WorkflowCard({ item }: { item: BuiltinWorkflowVm }) {
  return (
    <article className="lite-card workflow-card">
      <span className="lite-kind">内置模拟能力</span>
      <h2>{item.label}</h2>
      <p>{item.description}</p>
      <ol className="workflow-steps">
        {item.steps.map((step, index) => (
          <li key={step}>
            <b>{index + 1}</b>
            {step}
          </li>
        ))}
      </ol>
      <div className="workflow-card-footer">
        <span className="lite-badge enabled">{item.statusLabel}</span>
        <span>{item.resultLabel}</span>
      </div>
    </article>
  );
}

export function WorkflowsPage() {
  const [workflows, setWorkflows] = useState<BuiltinWorkflowVm[] | null>(null);
  const [error, setError] = useState<ApiError | null>(null);

  const load = useCallback(async (signal?: AbortSignal) => {
    setWorkflows(null);
    setError(null);
    try {
      const workflowResult = await listBuiltinWorkflows({ signal });
      setWorkflows(workflowResult);
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.code === "cancelled")) {
        setError(cause instanceof ApiError ? cause : new ApiError("暂时无法读取内置流程，请稍后重试。", 0));
      }
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => void load(controller.signal), 0);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [load]);

  return (
    <main className="lite-main">
      <header>
        <h1>内置流程（只读）</h1>
        <p>本页仅展示内置模拟流程说明，不承载真实执行与运行管理。</p>
      </header>

      <section className="ui030-legacy-banner" aria-label="只读引导">
        <div>
          <h2>真实运行请前往流程中心</h2>
          <p>
            本页只展示内置模拟流程。真实执行记录、状态追踪、取消和重试统一在流程中心（/workflow-runs）管理，不再使用本页查看最近运行或运行统计。
          </p>
        </div>
        <Link href="/workflow-runs">前往流程中心</Link>
      </section>

      {error ? (
        <ErrorState error={error} retry={() => void load()} />
      ) : !workflows ? (
        <div className="lite-loading" role="status">
          正在加载内置流程…
        </div>
      ) : (
        <section>
          <div className="lite-section-heading">
            <h2>内置模拟流程</h2>
          </div>
          {workflows.length ? (
            <div className="lite-grid">
              {workflows.map(item => (
                <WorkflowCard key={item.id} item={item} />
              ))}
            </div>
          ) : (
            <section className="lite-state">
              <h2>暂无内置流程</h2>
              <p>当前没有可展示的内置流程。</p>
            </section>
          )}
        </section>
      )}
    </main>
  );
}
