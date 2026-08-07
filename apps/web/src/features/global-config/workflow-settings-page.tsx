"use client";

import "./connection-settings.css";
import { useCallback, useEffect, useState } from "react";
import { ApiError } from "@/lib/api";
import { listConnectionTypes, listConnections, type ConnectionTypeDto, type WorkflowConnectionDto } from "./workflow-connection-api";
import {
  createWorkflow,
  getWorkflow,
  listWorkflows,
  mapWorkflow,
  setWorkflowEnabled,
  updateWorkflow,
  validateWorkflow,
  verifyWorkflow,
  type ApplicableStage,
  type LlmStrategy,
  type WorkflowDto,
  type WorkflowForm,
  type WorkflowVm
} from "./workflow-api";
import { listLlmProviders, type LlmProviderDto, type ValidationStatus } from "./llm-provider-api";

type Drawer = { mode: "create" } | { mode: "edit"; workflow: WorkflowDto } | null;
type Errors = Record<string, string>;

const stages: [ApplicableStage, string][] = [
  ["chapter_planning", "章节规划"],
  ["content_generation", "内容生成"],
  ["review", "审核"],
  ["rewrite", "重写"]
];

const blank = (connectionId = ""): WorkflowForm => ({
  name: "",
  connectionId,
  applicableStages: [],
  referenceType: "workflow_id",
  referenceValue: "",
  inputContractVersion: "v1",
  outputContractVersion: "v1",
  defaultParametersJson: "{}",
  note: "",
  llmStrategy: "none",
  llmProviderId: "",
  llmModel: ""
});

const formOf = (workflow: WorkflowDto): WorkflowForm => ({
  name: workflow.name,
  connectionId: workflow.connectionId,
  applicableStages: workflow.applicableStages,
  referenceType: workflow.typeConfig.referenceType,
  referenceValue: workflow.typeConfig.referenceValue,
  inputContractVersion: workflow.inputContractVersion,
  outputContractVersion: workflow.outputContractVersion,
  defaultParametersJson: JSON.stringify(workflow.defaultParameters, null, 2),
  note: workflow.note ?? "",
  llmStrategy: workflow.llmStrategy ?? "none",
  llmProviderId: workflow.llmProviderId ?? "",
  llmModel: workflow.llmModel ?? ""
});

const fieldErrors = (error: ApiError): Errors => {
  const fields = error.details.fields;
  return fields && typeof fields === "object" && !Array.isArray(fields)
    ? Object.fromEntries(
        Object.entries(fields as Record<string, unknown>).map(([key, value]) => [
          key,
          typeof value === "string" ? value : "输入不符合要求。"
        ])
      )
    : {};
};

function validationBadgeText(status: ValidationStatus): string {
  switch (status) {
    case "verified":
      return "验证成功";
    case "stale":
      return "配置已变更";
    case "failed":
      return "依赖失效";
    case "verifying":
      return "验证中";
    default:
      return "未验证";
  }
}

function WorkflowDrawer({
  state,
  connections,
  onClose,
  onSaved
}: {
  state: Exclude<Drawer, null>;
  connections: WorkflowConnectionDto[];
  onClose: () => void;
  onSaved: () => Promise<void>;
}) {
  const editing = state.mode === "edit";
  const [current, setCurrent] = useState<WorkflowDto | null>(editing ? state.workflow : null);
  const [form, setForm] = useState<WorkflowForm>(() =>
    editing ? formOf(state.workflow) : blank(connections[0]?.id)
  );
  const [providers, setProviders] = useState<LlmProviderDto[]>([]);
  const [error, setError] = useState<string>();
  const [errors, setErrors] = useState<Errors>({});
  const [conflict, setConflict] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    void listLlmProviders({ limit: 100 }, { signal: controller.signal })
      .then(result => setProviders(result.items))
      .catch(() => setProviders([]));
    return () => controller.abort();
  }, []);

  const set = <K extends keyof WorkflowForm>(key: K, value: WorkflowForm[K]) => {
    setForm(currentForm => {
      const next = { ...currentForm, [key]: value };
      if (key === "llmStrategy") {
        if (value === "none" || value === "n8n_managed") {
          next.llmProviderId = "";
          next.llmModel = "";
        }
      }
      if (key === "llmProviderId" && typeof value === "string") {
        const provider = providers.find(item => item.id === value);
        if (provider && !next.llmModel) next.llmModel = provider.defaultModel;
      }
      return next;
    });
    setError(undefined);
    setErrors({});
  };

  const toggle = (stage: ApplicableStage) =>
    set(
      "applicableStages",
      form.applicableStages.includes(stage)
        ? form.applicableStages.filter(value => value !== stage)
        : [...form.applicableStages, stage]
    );

  const reload = async () => {
    if (!editing || !current) return;
    setSaving(true);
    try {
      const workflow = await getWorkflow(current.id);
      setCurrent(workflow);
      setForm(formOf(workflow));
      setConflict(false);
      setErrors({});
      setError("已重新加载服务端最新数据，请确认后重新保存。");
    } catch {
      setError("暂时无法加载最新工作流。");
    } finally {
      setSaving(false);
    }
  };

  const persist = async (andVerify: boolean) => {
    if (saving) return;
    const invalid = validateWorkflow(form);
    if (invalid) {
      setError(invalid);
      return;
    }
    setSaving(true);
    setError(undefined);
    setErrors({});
    try {
      let saved: WorkflowDto;
      if (editing && current) {
        saved = await updateWorkflow(current.id, form, current.version, crypto.randomUUID());
      } else {
        saved = await createWorkflow(form, crypto.randomUUID());
      }
      if (andVerify) {
        await verifyWorkflow(saved.id, saved.version, crypto.randomUUID());
        saved = await getWorkflow(saved.id);
      }
      setCurrent(saved);
      setForm(formOf(saved));
      await onSaved();
      if (!andVerify) onClose();
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      if (api?.status === 409 || api?.code === "version_conflict") {
        setConflict(true);
        setError("该工作流已被其他操作更新，请重新加载后再保存。");
      } else {
        setErrors(api ? fieldErrors(api) : {});
        setError(andVerify ? "保存或验证失败，请检查输入后重试。" : "保存失败，请检查输入后重试。");
      }
    } finally {
      setSaving(false);
    }
  };

  const statusKey = current?.validationStatus ?? "unverified";
  const isExecutable = current?.executable === true;
  const reasons = current?.ineligibilityReasons?.map(item => item.message) ?? [];
  const safeError = current?.safeError?.message ?? current?.lastErrorMessage ?? null;

  return (
    <div className="llm-drawer-backdrop ui008-drawer-backdrop" onClick={onClose}>
      <aside
        className="llm-drawer ui008-drawer"
        role="dialog"
        aria-modal="true"
        aria-label={editing ? "编辑工作流" : "添加工作流"}
        onClick={event => event.stopPropagation()}
      >
        <header className="ui008-header">
          <div>
            <h2>{editing ? "编辑工作流" : "添加工作流"}</h2>
            <p>LLM 策略固定在 Workflow Configuration 层，项目不可覆盖。</p>
          </div>
          <button type="button" aria-label="关闭" onClick={onClose} disabled={saving}>
            ×
          </button>
        </header>

        <form
          className="ui008-form"
          onSubmit={event => {
            event.preventDefault();
            void persist(false);
          }}
        >
          <div className="llm-drawer-body ui008-body">
            <section className="ui008-section">
              <h3>基本信息</h3>
              <label>
                工作流名称 *
                <input
                  value={form.name}
                  maxLength={160}
                  placeholder="例如：标准章节规划工作流"
                  aria-invalid={Boolean(errors.name)}
                  onChange={event => set("name", event.target.value)}
                  disabled={saving}
                />
                {errors.name && <small className="llm-field-error">{errors.name}</small>}
              </label>
              <div className="ui008-grid-2">
                <label>
                  关联连接 *
                  <select
                    value={form.connectionId}
                    aria-invalid={Boolean(errors.connectionId)}
                    onChange={event => set("connectionId", event.target.value)}
                    disabled={saving}
                  >
                    {connections.map(connection => (
                      <option key={connection.id} value={connection.id}>
                        {connection.name}
                      </option>
                    ))}
                  </select>
                  {errors.connectionId && <small className="llm-field-error">{errors.connectionId}</small>}
                </label>
                <label>
                  工作流类型
                  <input value={editing && current ? current.workflowType : "选择连接后自动识别"} disabled />
                </label>
              </div>
            </section>

            <section className="ui008-section">
              <h3>业务契约</h3>
              <fieldset className="llm-check-group ui008-stages">
                <legend>适用环节 *</legend>
                {stages.map(([value, label]) => (
                  <label key={value}>
                    <input
                      type="checkbox"
                      checked={form.applicableStages.includes(value)}
                      onChange={() => toggle(value)}
                      disabled={saving}
                    />
                    {label}
                  </label>
                ))}
                {errors.applicableStages && <small className="llm-field-error">{errors.applicableStages}</small>}
              </fieldset>
              <div className="ui008-grid-2">
                <label>
                  引用类型 *
                  <select
                    value={form.referenceType}
                    onChange={event => set("referenceType", event.target.value as WorkflowForm["referenceType"])}
                    disabled={saving}
                  >
                    <option value="workflow_id">工作流 ID</option>
                    <option value="webhook_path">Webhook 路径</option>
                  </select>
                </label>
                <label>
                  引用值 *
                  <input
                    value={form.referenceValue}
                    maxLength={512}
                    placeholder="例如：wf_prod_xxx"
                    aria-invalid={Boolean(errors.typeConfig)}
                    onChange={event => set("referenceValue", event.target.value)}
                    disabled={saving}
                  />
                  {errors.typeConfig && <small className="llm-field-error">{errors.typeConfig}</small>}
                </label>
              </div>
              <div className="ui008-grid-2">
                <label>
                  输入契约版本 *
                  <input
                    value={form.inputContractVersion}
                    maxLength={40}
                    onChange={event => set("inputContractVersion", event.target.value)}
                    disabled={saving}
                  />
                </label>
                <label>
                  输出契约版本 *
                  <input
                    value={form.outputContractVersion}
                    maxLength={40}
                    onChange={event => set("outputContractVersion", event.target.value)}
                    disabled={saving}
                  />
                </label>
              </div>
              <label>
                默认参数（JSON）
                <textarea
                  value={form.defaultParametersJson}
                  aria-invalid={Boolean(errors.defaultParameters)}
                  onChange={event => set("defaultParametersJson", event.target.value)}
                  disabled={saving}
                />
                {errors.defaultParameters && <small className="llm-field-error">{errors.defaultParameters}</small>}
              </label>
              <label>
                备注（可选）
                <textarea
                  value={form.note}
                  maxLength={5000}
                  placeholder="添加备注信息…"
                  onChange={event => set("note", event.target.value)}
                  disabled={saving}
                />
              </label>
            </section>

            <section className="ui008-section">
              <div className="ui008-section-head">
                <h3>LLM 使用策略</h3>
                <span className="ui008-chip">项目绑定后不可覆盖 Provider、模型或策略</span>
              </div>
              <div className="ui008-strategy-options">
                {(
                  [
                    ["acf_managed", "ACF 托管模型"],
                    ["n8n_managed", "n8n 内部模型"],
                    ["none", "不使用模型"]
                  ] as const
                ).map(([value, label]) => (
                  <label key={value} className={`ui008-strategy-option ${form.llmStrategy === value ? "active" : ""}`}>
                    <input
                      type="radio"
                      name="llmStrategy"
                      checked={form.llmStrategy === value}
                      onChange={() => set("llmStrategy", value)}
                      disabled={saving}
                    />
                    {label}
                  </label>
                ))}
              </div>
              {form.llmStrategy === "acf_managed" && (
                <div className="ui008-grid-2 ui008-provider-block">
                  <label>
                    Provider *
                    <select
                      value={form.llmProviderId}
                      onChange={event => set("llmProviderId", event.target.value)}
                      disabled={saving || providers.length === 0}
                    >
                      <option value="">请选择 Provider</option>
                      {providers.map(provider => (
                        <option key={provider.id} value={provider.id}>
                          {provider.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Model *
                    <input
                      value={form.llmModel}
                      placeholder="输入模型名称"
                      onChange={event => set("llmModel", event.target.value)}
                      disabled={saving}
                    />
                  </label>
                </div>
              )}
              {form.llmStrategy !== "acf_managed" && (
                <p className="ui008-strategy-hint">
                  {form.llmStrategy === "n8n_managed"
                    ? "由 n8n 工作流内部管理模型，不在此处绑定 Provider。"
                    : "当前工作流不使用模型。"}
                </p>
              )}
            </section>

            {editing && current && (
              <section className="ui008-section ui008-status-section">
                <h3>状态</h3>
                <div className="ui008-status-grid">
                  <div>
                    <span className="ui008-status-label">验证状态</span>
                    <span className={`ui004-badge val-badge-${statusKey}`}>{validationBadgeText(statusKey)}</span>
                  </div>
                  <div>
                    <span className="ui008-status-label">启用状态</span>
                    <strong>{current.enabled ? "启用中" : "已停用"}</strong>
                  </div>
                  <div>
                    <span className="ui008-status-label">执行资格</span>
                    <strong className={isExecutable ? "ready" : "blocked"}>
                      {isExecutable ? "可执行" : "不可执行"}
                    </strong>
                  </div>
                  <div>
                    <span className="ui008-status-label">版本</span>
                    <strong className="ui007-version">v{current.version}</strong>
                  </div>
                </div>
                <p className="ui008-status-note">
                  编辑关键字段将产生新版本并使验证失效；启用状态和项目绑定会保留，需重新验证通过后恢复执行资格。
                </p>
                {(safeError || reasons[0]) && (
                  <div className="ui005-alert error" role="alert">
                    <strong>不可执行原因</strong>
                    <span>{safeError || reasons[0]}</span>
                  </div>
                )}
              </section>
            )}

            {error && (
              <div className="llm-form-error" role="alert">
                {error}
                {conflict && (
                  <button type="button" onClick={() => void reload()} disabled={saving}>
                    重新加载
                  </button>
                )}
              </div>
            )}
          </div>

          <footer className="ui008-footer">
            <button type="button" className="ui008-btn secondary" onClick={onClose} disabled={saving}>
              取消
            </button>
            <button type="submit" className="ui008-btn secondary" disabled={saving}>
              {saving ? "保存中…" : "保存"}
            </button>
            <button
              type="button"
              className="ui008-btn primary"
              disabled={saving}
              onClick={() => void persist(true)}
            >
              {saving ? "处理中…" : "保存并验证"}
            </button>
          </footer>
        </form>
      </aside>
    </div>
  );
}

export function WorkflowSettingsPage() {
  const [workflows, setWorkflows] = useState<WorkflowVm[] | null>(null);
  const [connections, setConnections] = useState<WorkflowConnectionDto[] | null>(null);
  const [types, setTypes] = useState<ConnectionTypeDto[] | null>(null);
  const [total, setTotal] = useState(0);
  const [query, setQuery] = useState("");
  const [applicableStage, setApplicableStage] = useState("");
  const [llmStrategy, setLlmStrategy] = useState("");
  const [validationStatus, setValidationStatus] = useState("");
  const [enabled, setEnabled] = useState("");
  const [executable, setExecutable] = useState("");
  const [offset, setOffset] = useState(0);
  const [drawer, setDrawer] = useState<Drawer>(null);
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [verifyingId, setVerifyingId] = useState<string | null>(null);
  const limit = 20;

  const load = useCallback(
    async (signal?: AbortSignal) => {
      setError(undefined);
      try {
        const [result, connectionResult, typeResult] = await Promise.all([
          listWorkflows(
            {
              q: query || undefined,
              applicableStage: (applicableStage || undefined) as ApplicableStage | undefined,
              llmStrategy: (llmStrategy || undefined) as LlmStrategy | undefined,
              validationStatus: (validationStatus || undefined) as ValidationStatus | undefined,
              enabled: enabled === "true" ? true : enabled === "false" ? false : undefined,
              executable: executable === "true" ? true : executable === "false" ? false : undefined,
              limit,
              offset
            },
            { signal }
          ),
          listConnections({ limit: 100 }, { signal }),
          listConnectionTypes({ signal })
        ]);
        if (!signal?.aborted) {
          setWorkflows(result.items.map(mapWorkflow));
          setConnections(connectionResult.items);
          setTypes(typeResult.items);
          setTotal(result.total);
        }
      } catch (cause) {
        if (!(cause instanceof ApiError && cause.code === "cancelled")) {
          setError("暂时无法加载工作流配置，请稍后重试。");
        }
      }
    },
    [applicableStage, enabled, executable, llmStrategy, offset, query, validationStatus]
  );

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => void load(controller.signal), 300);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [load]);

  const reset = (action: () => void) => {
    setOffset(0);
    action();
  };

  const edit = async (workflow: WorkflowVm) => {
    try {
      setDrawer({ mode: "edit", workflow: await getWorkflow(workflow.id) });
    } catch {
      setNotice("暂时无法读取工作流详情。");
    }
  };

  const handleToggleEnabled = async (item: WorkflowVm) => {
    if (togglingId) return;
    setTogglingId(item.id);
    try {
      await setWorkflowEnabled(item.id, item.version, !item.enabled, crypto.randomUUID());
      await load();
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setError(api?.message || "启停失败，请稍后重试。");
    } finally {
      setTogglingId(null);
    }
  };

  const handleVerify = async (item: WorkflowVm) => {
    if (verifyingId) return;
    setVerifyingId(item.id);
    try {
      await verifyWorkflow(item.id, item.version, crypto.randomUUID());
      await load();
      setNotice("验证已完成。");
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setError(api?.message || "验证失败，请稍后重试。");
    } finally {
      setVerifyingId(null);
    }
  };

  const pages = Math.max(1, Math.ceil(total / limit));
  const page = Math.floor(offset / limit) + 1;

  return (
    <section className="settings-content ui007-workflow-list">
      <div className="ui007-notice-banner">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="ui004-info-icon">
          <circle cx="12" cy="12" r="10" />
          <line x1="12" y1="16" x2="12" y2="12" />
          <line x1="12" y1="8" x2="12.01" y2="8" />
        </svg>
        <span>
          工作流通过连接、业务契约和 LLM 策略校验后获得执行资格。修改关键字段后保留启用状态和项目绑定，但执行资格失效，重新验证后恢复。
        </span>
      </div>

      {notice && (
        <p className="llm-toast" role="status">
          {notice}
        </p>
      )}

      {error ? (
        <div className="llm-state" role="alert">
          <h3>暂时无法加载</h3>
          <p>{error}</p>
          <button onClick={() => void load()}>重试</button>
        </div>
      ) : !workflows || !connections || !types ? (
        <div className="llm-loading" role="status">
          正在加载工作流配置…
        </div>
      ) : !connections.length ? (
        <div className="llm-state">
          <h3>请先添加连接</h3>
          <p>工作流必须关联一个已保存的执行连接。</p>
        </div>
      ) : (
        <>
          <div className="ui007-toolbar">
            <div className="ui007-filters">
              <div className="ui004-search-wrap">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="ui004-search-icon">
                  <circle cx="11" cy="11" r="8" />
                  <line x1="21" y1="21" x2="16.65" y2="16.65" />
                </svg>
                <input
                  className="ui004-search-input"
                  value={query}
                  onChange={event => reset(() => setQuery(event.target.value))}
                  placeholder="搜索名称或 ID"
                />
              </div>
              <select
                className="ui004-filter-select"
                value={applicableStage}
                onChange={event => reset(() => setApplicableStage(event.target.value))}
              >
                <option value="">适用环节</option>
                {stages.map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
              <select
                className="ui004-filter-select"
                value={llmStrategy}
                onChange={event => reset(() => setLlmStrategy(event.target.value))}
              >
                <option value="">LLM 策略</option>
                <option value="acf_managed">ACF 托管</option>
                <option value="n8n_managed">n8n 内部模型</option>
                <option value="none">不使用模型</option>
              </select>
              <select
                className="ui004-filter-select"
                value={validationStatus}
                onChange={event => reset(() => setValidationStatus(event.target.value))}
              >
                <option value="">验证状态</option>
                <option value="unverified">未验证</option>
                <option value="verified">验证成功</option>
                <option value="stale">配置已变更</option>
                <option value="failed">验证失败</option>
              </select>
              <select
                className="ui004-filter-select"
                value={enabled}
                onChange={event => reset(() => setEnabled(event.target.value))}
              >
                <option value="">启用状态</option>
                <option value="true">已启用</option>
                <option value="false">未启用</option>
              </select>
              <select
                className="ui004-filter-select"
                value={executable}
                onChange={event => reset(() => setExecutable(event.target.value))}
              >
                <option value="">执行资格</option>
                <option value="true">可执行</option>
                <option value="false">不可执行</option>
              </select>
            </div>
            <button
              className="ui004-add-btn primary"
              onClick={() => setDrawer({ mode: "create" })}
              disabled={!connections.length}
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="6" y1="12" x2="18" y2="12" />
              </svg>
              添加工作流
            </button>
          </div>

          {workflows.length === 0 ? (
            <div className="llm-state">
              <h3>暂无工作流配置</h3>
              <p>添加工作流后，可供后续项目绑定使用。</p>
            </div>
          ) : (
            <div className="ui007-table-wrap">
              <table className="ui007-table">
                <thead>
                  <tr>
                    <th>工作流名称</th>
                    <th>关联连接</th>
                    <th>适用环节</th>
                    <th>LLM 策略</th>
                    <th>验证状态</th>
                    <th>启用状态</th>
                    <th>执行资格</th>
                    <th>版本</th>
                    <th style={{ textAlign: "right" }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {workflows.map(item => {
                    const statusKey = item.validationStatus;
                    const rowTone =
                      statusKey === "failed" ? "row-failed" : statusKey === "stale" ? "row-stale" : "";
                    return (
                      <tr key={item.id} className={rowTone}>
                        <td>
                          <div className="ui007-name-cell">
                            <strong>{item.name}</strong>
                            <span className="ui007-id-hint">{item.shortId}</span>
                          </div>
                        </td>
                        <td>
                          <div className="ui007-connection-cell">
                            <span>{item.connectionName}</span>
                            <span className="ui007-type-hint">{item.connectionTypeLabel}</span>
                          </div>
                        </td>
                        <td>{item.stagesLabel}</td>
                        <td>
                          <div className="ui007-strategy-cell">
                            <span>{item.strategyLabel}</span>
                            {item.strategyDetail && <span className="ui007-strategy-detail">{item.strategyDetail}</span>}
                          </div>
                        </td>
                        <td>
                          <div className="ui007-val-status">
                            <span className={`ui004-badge val-badge-${statusKey}`}>
                              {validationBadgeText(statusKey)}
                            </span>
                            {statusKey === "failed" && item.reasons[0] && (
                              <span className="ui004-sub-text error">{item.reasons[0]}</span>
                            )}
                            {statusKey === "stale" && (
                              <span className="ui004-sub-text">原验证已失效</span>
                            )}
                          </div>
                        </td>
                        <td>
                          <button
                            type="button"
                            className={`ui004-toggle-switch ${item.enabled ? "enabled" : "disabled"}`}
                            onClick={() => void handleToggleEnabled(item)}
                            disabled={togglingId === item.id}
                            title={item.enabled ? "点击停用" : "点击启用"}
                          >
                            <span className="ui004-switch-thumb" />
                          </button>
                        </td>
                        <td>
                          <span className={`ui004-eligibility ${item.executable ? "ready" : "blocked"}`}>
                            <span className="ui004-dot" />
                            {item.executable ? "可执行" : "不可执行"}
                          </span>
                        </td>
                        <td>
                          <span className="ui007-version">v{item.version}</span>
                        </td>
                        <td style={{ textAlign: "right" }}>
                          <div className="ui004-actions">
                            <button className="ui004-action-btn" onClick={() => void edit(item)}>
                              编辑
                            </button>
                            <button
                              className="ui004-action-btn"
                              onClick={() => void handleVerify(item)}
                              disabled={verifyingId === item.id}
                            >
                              {verifyingId === item.id
                                ? "验证中…"
                                : statusKey === "stale" || statusKey === "failed"
                                  ? "重新验证"
                                  : "验证"}
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}

          <div className="llm-pagination">
            <span>
              第 {page} / {pages} 页，共 {total} 条
            </span>
            <button disabled={offset === 0} onClick={() => setOffset(value => Math.max(0, value - limit))}>
              上一页
            </button>
            <button disabled={offset + limit >= total} onClick={() => setOffset(value => value + limit)}>
              下一页
            </button>
          </div>
        </>
      )}

      {drawer && connections && (
        <WorkflowDrawer
          state={drawer}
          connections={connections}
          onClose={() => setDrawer(null)}
          onSaved={async () => {
            await load();
            setNotice("工作流已保存，并已同步服务端最新数据。");
          }}
        />
      )}
    </section>
  );
}
