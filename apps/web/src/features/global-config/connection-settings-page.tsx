"use client";

import "./connection-settings.css";
import { useCallback, useEffect, useState } from "react";
import { ApiError } from "@/lib/api";
import { createConnection, getConnection, listConnections, listConnectionTypes, mapConnection, setConnectionEnabled, updateConnection, validateConnectionForm, verifyConnection, type ConnectionFormInput, type ConnectionTypeDto, type ConnectionVm, type WorkflowConnectionDto } from "./workflow-connection-api";

type Drawer = { mode: "create" } | { mode: "edit"; item: WorkflowConnectionDto } | null;
const blank = (connectionType = ""): ConnectionFormInput => ({ name: "", connectionType, baseUrl: "", timeoutSeconds: 60, typeConfigJson: '{"referenceType":"workflow_id","referenceValue":""}', credential: "" });

function ConnectionDrawer({ drawer, types, onClose, onSaved }: { drawer: Exclude<Drawer, null>; types: ConnectionTypeDto[]; onClose: () => void; onSaved: () => Promise<void> }) {
  const editing = drawer.mode === "edit";
  const [form, setForm] = useState<ConnectionFormInput>(() => editing ? { name: drawer.item.name, connectionType: drawer.item.connectionType, baseUrl: drawer.item.baseUrl, timeoutSeconds: drawer.item.timeoutSeconds, typeConfigJson: JSON.stringify(drawer.item.typeConfig), credential: "" } : blank(types[0]?.connectionType));
  const [message, setMessage] = useState<string>(); const [busy, setBusy] = useState(false);
  const set = <K extends keyof ConnectionFormInput>(key: K, value: ConnectionFormInput[K]) => setForm(current => ({ ...current, [key]: value }));
  const save = async (event: React.FormEvent) => { event.preventDefault(); const invalid = validateConnectionForm(form, editing); if (invalid) return setMessage(invalid); setBusy(true); try { if (editing) await updateConnection(drawer.item.id, { ...form, expectedVersion: drawer.item.version }, crypto.randomUUID()); else await createConnection(form, crypto.randomUUID()); await onSaved(); onClose(); } catch (cause) { setMessage(cause instanceof ApiError && cause.status === 409 ? "Configuration changed. Reload and try again." : "Save failed. Please try again."); } finally { setBusy(false); } };
  const command = async (enabled?: boolean) => { if (!editing || busy) return; setBusy(true); try { if (enabled === undefined) await verifyConnection(drawer.item.id, drawer.item.version, crypto.randomUUID()); else await setConnectionEnabled(drawer.item.id, drawer.item.version, enabled, crypto.randomUUID()); await onSaved(); setMessage("Latest status has been refreshed."); } catch (cause) { setMessage(cause instanceof ApiError && cause.status === 409 ? "Configuration changed. Reload and try again." : "Action failed. Please try again."); } finally { setBusy(false); } };
  const config = (() => { try { return JSON.parse(form.typeConfigJson) as { referenceType: "workflow_id" | "webhook_path"; referenceValue: string }; } catch { return { referenceType: "workflow_id" as const, referenceValue: "" }; } })();
  const setConfig = (key: "referenceType" | "referenceValue", value: string) => set("typeConfigJson", JSON.stringify({ ...config, [key]: value }));
  return <div className="llm-drawer-backdrop"><aside className="llm-drawer" role="dialog" aria-modal="true" aria-label={editing ? "Edit workflow connection" : "Add workflow connection"}><header><div><h2>{editing ? "Edit connection" : "Add connection"}</h2><p>Credentials are input-only and never returned to this page.</p></div><button type="button" aria-label="Close" onClick={onClose} disabled={busy}>×</button></header><form onSubmit={save}><div className="llm-drawer-body"><label>Name *<input value={form.name} maxLength={120} onChange={event => set("name", event.target.value)} disabled={busy}/></label><label>Type *<select value={form.connectionType} onChange={event => set("connectionType", event.target.value)} disabled={editing || busy}>{types.map(type => <option key={type.connectionType} value={type.connectionType}>{type.displayName}</option>)}</select></label><label>Base URL *<input type="url" value={form.baseUrl} onChange={event => set("baseUrl", event.target.value)} disabled={busy}/></label><label>API Key{editing ? "" : " *"}<input type="password" value={form.credential} autoComplete="new-password" onChange={event => set("credential", event.target.value)} disabled={busy}/><small>{editing && drawer.item.hasCredential ? "Saved; leaving blank retains it." : "Stored securely and never shown again."}</small></label><label>Timeout (seconds)<input type="number" min="5" max="300" value={form.timeoutSeconds} onChange={event => set("timeoutSeconds", Number(event.target.value))} disabled={busy}/></label><label>Reference type<select value={config.referenceType} onChange={event => setConfig("referenceType", event.target.value)} disabled={busy}><option value="workflow_id">Workflow ID</option><option value="webhook_path">Webhook path</option></select></label><label>Reference value *<input value={config.referenceValue} maxLength={512} onChange={event => setConfig("referenceValue", event.target.value)} disabled={busy}/></label>{editing && <div className="llm-deferred"><button type="button" disabled={busy} onClick={() => void command()}>Verify connection</button><button type="button" disabled={busy} onClick={() => void command(!drawer.item.enabled)}>{drawer.item.enabled ? "Disable" : "Enable"}</button><p>Save, verification, and enablement are separate actions.</p></div>}{message && <div className="llm-form-error" role="alert">{message}</div>}</div><footer><button type="button" onClick={onClose} disabled={busy}>Cancel</button><button className="primary" disabled={busy}>{busy ? "Saving…" : "Save"}</button></footer></form></aside></div>;
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
  const [items, setItems] = useState<ConnectionVm[] | null>(null);
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
        setItems(result.items.map(item => mapConnection(item, catalogue.items)));
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

  const edit = async (item: ConnectionVm) => {
    try {
      setDrawer({ mode: "edit", item: await getConnection(item.id) });
    } catch {
      setError("暂时无法加载该连接详情。");
    }
  };

  const handleToggleEnabled = async (item: ConnectionVm) => {
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

  const handleVerify = async (item: ConnectionVm) => {
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
