"use client";

import "./connection-settings.css";
import { useCallback, useEffect, useState } from "react";
import { ApiError } from "@/lib/api";
import { createConnection, getConnection, listConnections, listConnectionTypes, mapConnection, setConnectionEnabled, updateConnection, validateConnectionForm, verifyConnection, type ConnectionFormInput, type ConnectionTypeDto, type ConnectionVm, type WorkflowConnectionDto } from "./workflow-connection-api";
import type { ValidationStatus } from "./llm-provider-api";

type Drawer = { mode: "create" } | { mode: "edit"; item: WorkflowConnectionDto } | null;
const blank = (connectionType = ""): ConnectionFormInput => ({ name: "", connectionType, baseUrl: "", timeoutSeconds: 60, typeConfigJson: '{"referenceType":"workflow_id","referenceValue":""}', credential: "" });

/** Page-layer list display model: keeps API mapper contract-stable and only adds UI display fields. */
type ConnectionListItem = ConnectionVm & {
  validationStatus: ValidationStatus;
  enabled: boolean;
  lastVerifiedAt: string | null;
};

function toConnectionListItem(item: WorkflowConnectionDto, types: ConnectionTypeDto[]): ConnectionListItem {
  const vm = mapConnection(item, types);
  return {
    ...vm,
    validationStatus: item.validationStatus ?? "unverified",
    enabled: item.enabled,
    lastVerifiedAt: item.lastVerifiedAt
  };
}

function ConnectionDrawer({
  drawer,
  types,
  onClose,
  onSaved
}: {
  drawer: Exclude<Drawer, null>;
  types: ConnectionTypeDto[];
  onClose: () => void;
  onSaved: () => Promise<void>;
}) {
  const editing = drawer.mode === "edit";
  const item = editing ? drawer.item : null;
  const [form, setForm] = useState<ConnectionFormInput>(() =>
    editing
      ? {
          name: drawer.item.name,
          connectionType: drawer.item.connectionType,
          baseUrl: drawer.item.baseUrl,
          timeoutSeconds: drawer.item.timeoutSeconds,
          typeConfigJson: JSON.stringify(drawer.item.typeConfig || { referenceType: "workflow_id", referenceValue: "default" }),
          credential: ""
        }
      : {
          name: "",
          connectionType: types[0]?.connectionType || "n8n",
          baseUrl: "",
          timeoutSeconds: 60,
          typeConfigJson: JSON.stringify({ referenceType: "workflow_id", referenceValue: "default" }),
          credential: ""
        }
  );

  const [message, setMessage] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [authType, setAuthType] = useState("api_key");

  const set = <K extends keyof ConnectionFormInput>(key: K, value: ConnectionFormInput[K]) =>
    setForm(current => ({ ...current, [key]: value }));

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    const invalid = validateConnectionForm(form, editing);
    if (invalid) return setMessage(invalid);
    setBusy(true);
    try {
      if (editing && item) {
        await updateConnection(item.id, { ...form, expectedVersion: item.version }, crypto.randomUUID());
      } else {
        await createConnection(form, crypto.randomUUID());
      }
      await onSaved();
      onClose();
    } catch (cause) {
      setMessage(
        cause instanceof ApiError && cause.status === 409
          ? "配置已在外部被更新，请刷新页面重试。"
          : "保存失败，请稍后重试。"
      );
    } finally {
      setBusy(false);
    }
  };

  const handleVerify = async () => {
    if (!editing || !item || busy) return;
    setBusy(true);
    try {
      await verifyConnection(item.id, item.version, crypto.randomUUID());
      await onSaved();
      setMessage("验证已完成。");
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setMessage(api?.message || "验证失败，请核对地址与凭据。");
    } finally {
      setBusy(false);
    }
  };

  const handleToggleEnabled = async () => {
    if (!editing || !item || busy) return;
    setBusy(true);
    try {
      await setConnectionEnabled(item.id, item.version, !item.enabled, crypto.randomUUID());
      await onSaved();
      setMessage(item.enabled ? "已停用连接。" : "已启用连接。");
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setMessage(api?.message || "操作失败，请稍后重试。");
    } finally {
      setBusy(false);
    }
  };

  const statusKey = item?.validationStatus || "unverified";
  const isVerified = statusKey === "verified";
  const isExecutable = item?.executable === true || (item?.enabled && isVerified);
  const impactCount = item ? (item.affectedWorkflowConfigurationCount ?? item.workflowConfigurationCount ?? 0) : 0;

  return (
    <div className="ui005-drawer-backdrop" onClick={onClose}>
      <aside
        className="ui005-drawer"
        role="dialog"
        aria-modal="true"
        aria-label={editing ? "编辑 Connection" : "新建 Connection"}
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <header className="ui005-drawer-header">
          <div>
            <div className="ui005-header-title">
              <h2>{editing ? "编辑 Connection" : "新建 Connection"}</h2>
              {item && <span className="ui005-id-tag">ID: {item.id.slice(0, 12)}…</span>}
            </div>
            <p className="ui005-header-desc">
              关键字段变化后保留启用状态和所有绑定，但执行资格立即失效。
            </p>
          </div>
          <button type="button" className="ui005-close-btn" onClick={onClose} disabled={busy} aria-label="关闭">
            ×
          </button>
        </header>

        <form onSubmit={save} className="ui005-drawer-form">
          <div className="ui005-drawer-body">
            {/* Section 1: 基本信息 */}
            <section className="ui005-section">
              <h3 className="ui005-section-title">
                <span className="ui005-title-bar" />
                基本信息
              </h3>
              <div className="ui005-grid-2">
                <div className="ui005-field">
                  <label>连接名称 <span className="ui005-req">*</span></label>
                  <input
                    className="ui005-input"
                    value={form.name}
                    onChange={e => set("name", e.target.value)}
                    placeholder="例如：生产 n8n"
                    disabled={busy}
                  />
                </div>
                <div className="ui005-field">
                  <label>类型</label>
                  <select className="ui005-select read-only" disabled value={form.connectionType}>
                    {types.map(t => <option key={t.connectionType} value={t.connectionType}>{t.displayName}</option>)}
                  </select>
                </div>
              </div>

              <div className="ui005-grid-12">
                <div className="ui005-field col-8">
                  <label>Base URL <span className="ui005-req">*</span></label>
                  <input
                    className="ui005-input mono"
                    value={form.baseUrl}
                    onChange={e => set("baseUrl", e.target.value)}
                    placeholder="https://n8n.example.com"
                    disabled={busy}
                  />
                </div>
                <div className="ui005-field col-4">
                  <label>请求超时 (秒)</label>
                  <input
                    className="ui005-input"
                    type="number"
                    min="5"
                    max="300"
                    value={form.timeoutSeconds}
                    onChange={e => set("timeoutSeconds", Number(e.target.value))}
                    disabled={busy}
                  />
                </div>
              </div>
            </section>

            <hr className="ui005-divider" />

            {/* Section 2: 凭据 */}
            <section className="ui005-section">
              <h3 className="ui005-section-title">
                <span className="ui005-title-bar" />
                凭据
              </h3>
              <div className="ui005-card p-4">
                <div className="ui005-field max-half mb-3">
                  <label>认证方式</label>
                  <select
                    className="ui005-select"
                    value={authType}
                    onChange={e => setAuthType(e.target.value)}
                    disabled={busy}
                  >
                    <option value="api_key">API Key</option>
                  </select>
                </div>

                {editing && item?.hasCredential && (
                  <div className="ui005-cred-saved-box">
                    <span className="ui005-cred-lock">🔒 API Key：已安全保存</span>
                    <span className="ui005-cred-sub">留空继续表示保留已有 Credential</span>
                  </div>
                )}

                <div className="ui005-field">
                  <label>
                    {editing ? "更新 API Key" : "API Key"} {!editing && <span className="ui005-req">*</span>}
                  </label>
                  <input
                    type="password"
                    className="ui005-input"
                    value={form.credential}
                    autoComplete="new-password"
                    onChange={e => set("credential", e.target.value)}
                    placeholder={editing ? "留空表示保留已有 API Key" : "请输入 API Key"}
                    disabled={busy}
                  />
                </div>
              </div>
            </section>

            {editing && item && (
              <>
                <hr className="ui005-divider" />

                {/* Section 3: 验证与执行资格 */}
                <section className="ui005-section">
                  <div className="ui005-section-header">
                    <h3 className="ui005-section-title">
                      <span className="ui005-title-bar" />
                      验证与执行资格
                    </h3>
                    <span className={`ui005-badge val-badge-${statusKey}`}>
                      {statusKey === "verified" && "✓ 验证成功"}
                      {statusKey === "stale" && "⚠️ 配置已变更"}
                      {statusKey === "failed" && "❌ 验证失败"}
                      {statusKey === "unverified" && "未验证"}
                    </span>
                  </div>

                  <div className="ui005-card overflow-hidden">
                    <div className="ui005-status-grid">
                      {/* 启用状态 */}
                      <div className="ui005-status-col">
                        <div className="ui005-status-top">
                          <span className="ui005-label">启用状态</span>
                          <button
                            type="button"
                            className={`ui004-toggle-switch ${item.enabled ? "enabled" : "disabled"}`}
                            onClick={() => void handleToggleEnabled()}
                            disabled={busy}
                          >
                            <span className="ui004-switch-thumb" />
                          </button>
                        </div>
                        <div className="ui005-status-val">
                          <span className={`ui004-dot ${item.enabled ? "green" : "gray"}`} />
                          <strong>{item.enabled ? "启用中" : "已停用"}</strong>
                        </div>
                        <p className="ui005-sub">{item.enabled ? "启用状态已保留" : "已手动停用"}</p>
                      </div>

                      {/* 执行资格 */}
                      <div className={`ui005-status-col ${!isExecutable ? "blocked-bg" : ""}`}>
                        <div className="ui005-status-top">
                          <span className="ui005-label">执行资格</span>
                          <span className="ui005-time-tag">
                            {item.lastVerifiedAt ? `上次验证: ${formatDateTime(item.lastVerifiedAt)}` : "尚未验证"}
                          </span>
                        </div>
                        <div className="ui005-status-val">
                          <span className={`ui004-dot ${isExecutable ? "green" : "amber"}`} />
                          <strong className={isExecutable ? "ready" : "blocked"}>
                            {isExecutable ? "可执行" : "不可执行"}
                          </strong>
                        </div>
                        <div className="mt-2">
                          <button
                            type="button"
                            className="ui005-inline-verify-btn"
                            onClick={() => void handleVerify()}
                            disabled={busy}
                          >
                            {statusKey === "stale" ? "重新验证连接" : "验证连接"}
                          </button>
                        </div>
                      </div>
                    </div>

                    <div className="ui005-status-notices">
                      {statusKey === "failed" && (
                        <div className="ui005-alert error">
                          <strong>验证失败 (自动拦截)</strong>
                          <span>{typeof item.safeError === "string" ? item.safeError : item.safeError?.message || item.lastErrorMessage || "认证失败，请更新凭据"}</span>
                        </div>
                      )}
                      {statusKey === "stale" && (
                        <div className="ui005-alert warning">
                          <strong>配置已变更</strong>
                          <span>原验证已失效，需重新验证成功后自动恢复执行资格。</span>
                        </div>
                      )}
                      <div className="ui005-alert info">
                        <span>修改 Base URL、认证方式或凭据会使原验证失效，但不会自动停用或解除工作流/项目绑定。</span>
                      </div>
                    </div>
                  </div>
                </section>

                <hr className="ui005-divider" />

                {/* Section 4: 依赖影响 */}
                <section className="ui005-impact-card">
                  <div className="ui005-impact-left">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="ui005-impact-icon">
                      <polygon points="12 2 2 7 12 12 22 7 12 2"/>
                      <polyline points="2 17 12 22 22 17"/>
                      <polyline points="2 12 12 17 22 12"/>
                    </svg>
                    <div>
                      <h4>依赖影响</h4>
                      <p>影响 {impactCount} 个工作流配置；关系保留，新运行暂被阻断。</p>
                    </div>
                  </div>
                </section>
              </>
            )}

            {message && <div className="llm-form-error mt-4" role="alert">{message}</div>}
          </div>

          {/* Section 5: 底部固定操作区 */}
          <footer className="ui005-drawer-footer">
            <div className="ui005-footer-left">
              {editing && item && (
                <button
                  type="button"
                  className={`ui005-btn ${item.enabled ? "btn-danger-outline" : "btn-secondary"}`}
                  onClick={() => void handleToggleEnabled()}
                  disabled={busy}
                >
                  {item.enabled ? "停用连接" : "启用连接"}
                </button>
              )}
            </div>
            <div className="ui005-footer-right">
              <button type="button" className="ui005-btn btn-secondary" onClick={onClose} disabled={busy}>
                取消
              </button>
              {editing && (
                <button
                  type="button"
                  className="ui005-btn btn-secondary"
                  onClick={() => void handleVerify()}
                  disabled={busy}
                >
                  {statusKey === "stale" ? "重新验证" : "验证"}
                </button>
              )}
              <button type="submit" className="ui005-btn btn-primary" disabled={busy}>
                {busy ? "保存中…" : "保存"}
              </button>
            </div>
          </footer>
        </form>
      </aside>
    </div>
  );
}

function formatDateTime(isoString: string | null): string {
  if (!isoString) return "—";
  try {
    const d = new Date(isoString);
    if (isNaN(d.getTime())) return "—";
    const pad = (n: number) => String(n).padStart(2, '0');
    const hours = pad(d.getHours());
    const minutes = pad(d.getMinutes());
    return `${hours}:${minutes}`;
  } catch {
    return "—";
  }
}

export function ConnectionSettingsPage() {
  const [types, setTypes] = useState<ConnectionTypeDto[] | null>(null);
  const [items, setItems] = useState<ConnectionListItem[] | null>(null);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string>();
  const [drawer, setDrawer] = useState<Drawer>(null);
  const [query, setQuery] = useState("");
  const [connectionType, setConnectionType] = useState("");
  const [validationStatus, setValidationStatus] = useState("");
  const [enabled, setEnabled] = useState("");
  const [executable, setExecutable] = useState("");
  const [offset, setOffset] = useState(0);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [verifyingId, setVerifyingId] = useState<string | null>(null);
  const limit = 20;

  const load = useCallback(async (signal?: AbortSignal) => {
    setError(undefined);
    try {
      const [catalogue, result] = await Promise.all([
        listConnectionTypes({ signal }),
        listConnections({
          q: query || undefined,
          connectionType: connectionType || undefined,
          validationStatus: (validationStatus || undefined) as any,
          enabled: enabled === "true" ? true : enabled === "false" ? false : undefined,
          executable: executable === "true" ? true : executable === "false" ? false : undefined,
          limit,
          offset
        }, { signal })
      ]);
      if (!signal?.aborted) {
        setTypes(catalogue.items);
        setItems(result.items.map(item => toConnectionListItem(item, catalogue.items)));
        setTotal(result.total);
      }
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.code === "cancelled")) setError("暂时无法加载连接列表。");
    }
  }, [connectionType, enabled, executable, offset, query, validationStatus]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => void load(controller.signal), 0);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [load]);

  const edit = async (item: ConnectionListItem) => {
    try {
      setDrawer({ mode: "edit", item: await getConnection(item.id) });
    } catch {
      setError("暂时无法加载该连接详情。");
    }
  };

  const handleToggleEnabled = async (item: ConnectionListItem) => {
    if (togglingId) return;
    setTogglingId(item.id);
    try {
      await setConnectionEnabled(item.id, item.version, !item.enabled, crypto.randomUUID());
      await load();
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setError(api?.message || "操作失败，请稍后重试。");
    } finally {
      setTogglingId(null);
    }
  };

  const handleVerify = async (item: ConnectionListItem) => {
    if (verifyingId) return;
    setVerifyingId(item.id);
    try {
      await verifyConnection(item.id, item.version, crypto.randomUUID());
      await load();
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setError(api?.message || "验证失败，请稍后重试。");
    } finally {
      setVerifyingId(null);
    }
  };

  return (
    <section className="settings-content ui004-connection-list">
      {/* 规则说明条 */}
      <div className="ui004-notice-banner">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="ui004-info-icon">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="16" x2="12" y2="12"/>
          <line x1="12" y1="8" x2="12.01" y2="8"/>
        </svg>
        <span>连接关键字段变化后保留启用状态、工作流配置和项目绑定，但执行资格立即失效；重新验证成功后自动恢复。</span>
      </div>

      {error ? (
        <div className="llm-state" role="alert">
          <p>{error}</p>
          <button onClick={() => void load()}>重试</button>
        </div>
      ) : !items || !types ? (
        <div className="llm-loading" role="status">正在加载连接列表…</div>
      ) : (
        <>
          <div className="ui004-toolbar">
            <div className="ui004-filters">
              <div className="ui004-search-wrap">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="ui004-search-icon">
                  <circle cx="11" cy="11" r="8"/>
                  <line x1="21" y1="21" x2="16.65" y2="16.65"/>
                </svg>
                <input
                  className="ui004-search-input"
                  value={query}
                  onChange={event => { setOffset(0); setQuery(event.target.value); }}
                  placeholder="搜索连接名称或地址"
                />
              </div>

              <select
                className="ui004-filter-select"
                value={connectionType}
                onChange={event => { setOffset(0); setConnectionType(event.target.value); }}
              >
                <option value="">连接类型</option>
                {types.map(t => <option key={t.connectionType} value={t.connectionType}>{t.displayName}</option>)}
              </select>

              <select
                className="ui004-filter-select"
                value={validationStatus}
                onChange={event => { setOffset(0); setValidationStatus(event.target.value); }}
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
                onChange={event => { setOffset(0); setEnabled(event.target.value); }}
              >
                <option value="">启用状态</option>
                <option value="true">已启用</option>
                <option value="false">未启用</option>
              </select>

              <select
                className="ui004-filter-select"
                value={executable}
                onChange={event => { setOffset(0); setExecutable(event.target.value); }}
              >
                <option value="">执行资格</option>
                <option value="true">可执行</option>
                <option value="false">不可执行</option>
              </select>
            </div>

            <button
              className="ui004-add-btn primary"
              onClick={() => setDrawer({ mode: "create" })}
              disabled={!types?.length}
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="12" y1="5" x2="12" y2="19"/>
                <line x1="6" y1="12" x2="18" y2="12"/>
              </svg>
              添加连接
            </button>
          </div>

          {items.length === 0 ? (
            <div className="llm-state">
              <h3>暂无 Connection</h3>
              <p>添加 Connection 后，可供工作流配置绑定调用。</p>
            </div>
          ) : (
            <div className="ui004-table-wrap">
              <table className="ui004-table">
                <thead>
                  <tr>
                    <th>连接名称</th>
                    <th>类型</th>
                    <th>Base URL</th>
                    <th>验证状态</th>
                    <th>启用状态</th>
                    <th>执行资格</th>
                    <th style={{ textAlign: "center" }}>关联工作流</th>
                    <th>最近验证</th>
                    <th style={{ textAlign: "right" }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map(item => {
                    const statusKey = item.validationStatus || "unverified";
                    return (
                      <tr key={item.id}>
                        <td>
                          <div className="ui004-name-cell">
                            <strong>{item.name}</strong>
                            {item.hasCredential && <span className="ui004-cred-hint">凭据已配置</span>}
                          </div>
                        </td>
                        <td><span className="ui004-type-badge">{item.typeLabel}</span></td>
                        <td><code className="ui004-url">{item.baseUrl}</code></td>
                        <td>
                          <div className="ui004-val-status">
                            <span className={`ui004-badge val-badge-${statusKey}`}>
                              {statusKey === "verified" && "✓ 验证成功"}
                              {statusKey === "stale" && "⚠️ 配置已变更"}
                              {statusKey === "failed" && "❌ 验证失败"}
                              {statusKey === "unverified" && "未验证"}
                              {statusKey === "verifying" && "验证中"}
                            </span>
                            {statusKey === "stale" && <span className="ui004-sub-text">原验证已失效</span>}
                            {statusKey === "failed" && (
                              <span className="ui004-sub-text error">
                                {item.safeError || "认证失败，请更新凭据"}
                              </span>
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
                        <td style={{ textAlign: "center" }}>
                          <span className="ui004-impact-count">{item.impactCount}</span>
                        </td>
                        <td>
                          <span className="ui004-time">{formatDateTime(item.lastVerifiedAt)}</span>
                        </td>
                        <td style={{ textAlign: "right" }}>
                          <div className="ui004-actions">
                            <button className="ui004-action-btn" onClick={() => void edit(item)}>编辑</button>
                            <button
                              className="ui004-action-btn"
                              onClick={() => void handleVerify(item)}
                              disabled={verifyingId === item.id}
                            >
                              {verifyingId === item.id ? "验证中…" : (statusKey === "stale" ? "重新验证" : "验证")}
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
            <button disabled={offset === 0} onClick={() => setOffset(value => Math.max(0, value - limit))}>上一页</button>
            <span>第 {Math.floor(offset / limit) + 1} 页，共 {Math.max(1, Math.ceil(total / limit))} 页</span>
            <button disabled={offset + limit >= total} onClick={() => setOffset(value => value + limit)}>下一页</button>
          </div>
        </>
      )}

      {drawer && types && (
        <ConnectionDrawer
          drawer={drawer}
          types={types}
          onClose={() => setDrawer(null)}
          onSaved={load}
        />
      )}
    </section>
  );
}
