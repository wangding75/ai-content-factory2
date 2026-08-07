"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError } from "@/lib/api";
import "./llm-settings.css";
import { createLlmProvider, discoverLlmProviderModels, getLlmProvider, listLlmProviderTypes, listLlmProviders, mapLlmProvider, setLlmProviderEnabled, updateLlmProvider, validateProviderForm, verifyLlmProvider, type LlmProviderDto, type LlmProviderType, type LlmProviderTypeDto, type LlmProviderVm, type ProviderFormInput, type ValidationStatus } from "@/features/global-config/llm-provider-api";

type Drawer = { mode: "create" } | { mode: "edit"; provider: LlmProviderDto } | null;
const blank = (providerType: LlmProviderType = "openai_compatible"): ProviderFormInput => ({ name: "", providerType, baseUrl: "", defaultModel: "", timeoutSeconds: 60, secret: "" });

function ProviderDrawer({ drawer, types, onClose, onSaved }: { drawer: Exclude<Drawer, null>; types: LlmProviderTypeDto[]; onClose: () => void; onSaved: () => Promise<void> }) {
  const editing = drawer.mode === "edit";
  const [providerState, setProviderState] = useState<LlmProviderDto | undefined>(editing ? drawer.provider : undefined);
  const initial = (): ProviderFormInput => editing && providerState ? { name: providerState.name, providerType: providerState.providerType, baseUrl: providerState.baseUrl, defaultModel: providerState.defaultModel, timeoutSeconds: providerState.timeoutSeconds, secret: "" } : blank(types[0]?.providerType);
  const [form, setForm] = useState(initial);
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);
  const [checking, setChecking] = useState(false);
  const [conflict, setConflict] = useState(false);
  const secretSupported = types.find(type => type.providerType === form.providerType)?.supportsSecret ?? true;

  const [showSecretInput, setShowSecretInput] = useState(false);
  const [modelSearch, setModelSearch] = useState("");
  const [manualModel, setManualModel] = useState("");
  const [manualValidationResult, setManualValidationResult] = useState<string | null>(null);

  interface DiscoveredModelItem {
    modelName: string;
    displayName: string;
    available: boolean;
  }
  const [discoveredModels, setDiscoveredModels] = useState<DiscoveredModelItem[]>([]);

  const [prevProviderId, setPrevProviderId] = useState<string | null>(editing && drawer.provider ? drawer.provider.id : null);
  const currentId = editing && drawer.provider ? drawer.provider.id : null;

  if (editing && drawer.provider && currentId !== prevProviderId) {
    setPrevProviderId(currentId);
    setProviderState(drawer.provider);
    const p = drawer.provider;
    setForm({
      name: p.name,
      providerType: p.providerType,
      baseUrl: p.baseUrl,
      defaultModel: p.defaultModel,
      timeoutSeconds: p.timeoutSeconds,
      secret: ""
    });
    setShowSecretInput(false);
    setDiscoveredModels([]);
    setModelSearch("");
    setManualModel("");
    setManualValidationResult(null);
  }

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !saving && !checking) {
        onClose();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [saving, checking, onClose]);

  useEffect(() => {
    if (editing && providerState?.id) {
      fetch(`/api/v1/llm-providers/${encodeURIComponent(providerState.id)}/models`)
        .then(r => r.json())
        .then(res => {
          if (res?.data && typeof res.data.total === "number") {
            setProviderState(prev => prev ? { ...prev, modelCount: res.data.total } : prev);
          }
        })
        .catch(() => {});
    }
  }, [editing, providerState?.id]);

  const set = <K extends keyof ProviderFormInput>(key: K, value: ProviderFormInput[K]) => { setForm(current => ({ ...current, [key]: value })); setError(undefined); };

  const reload = async () => {
    if (!editing || !providerState) return;
    setSaving(true);
    try {
      const latest = await getLlmProvider(providerState.id);
      setProviderState({ ...latest, modelCount: providerState.modelCount });
      setForm({ name: latest.name, providerType: latest.providerType, baseUrl: latest.baseUrl, defaultModel: latest.defaultModel, timeoutSeconds: latest.timeoutSeconds, secret: "" });
      setConflict(false);
      setError("已加载服务端最新配置，请确认后重新保存。");
    } catch {
      setError("暂时无法加载最新配置。");
    } finally {
      setSaving(false);
    }
  };

  const submit = async (event?: React.FormEvent | React.MouseEvent) => {
    event?.preventDefault();
    const invalid = validateProviderForm(form, secretSupported, editing);
    if (invalid) {
      setError(invalid);
      return;
    }
    setSaving(true);
    try {
      if (editing && providerState) {
        await updateLlmProvider(providerState.id, { name: form.name, baseUrl: form.baseUrl, defaultModel: form.defaultModel, timeoutSeconds: form.timeoutSeconds, secret: form.secret, expectedVersion: providerState.version }, crypto.randomUUID());
      } else {
        await createLlmProvider(form, crypto.randomUUID());
      }
      await onSaved();
      onClose();
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      if (api?.status === 409 || api?.code === "version_conflict") {
        setConflict(true);
        setError("该配置已被其他操作更新，请重新加载后再保存。");
      } else {
        setError("保存失败，请检查输入后重试。");
      }
    } finally {
      setSaving(false);
    }
  };

  const handleSaveAndVerify = async () => {
    if (!editing || !providerState) return;
    const invalid = validateProviderForm(form, secretSupported, editing);
    if (invalid) {
      setError(invalid);
      return;
    }
    setSaving(true);
    try {
      const updated = await updateLlmProvider(
        providerState.id,
        {
          name: form.name,
          baseUrl: form.baseUrl,
          defaultModel: form.defaultModel,
          timeoutSeconds: form.timeoutSeconds,
          secret: form.secret,
          expectedVersion: providerState.version
        },
        crypto.randomUUID()
      );
      await verifyLlmProvider(providerState.id, updated.version, crypto.randomUUID());
      await onSaved();
      onClose();
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      if (api?.status === 409 || api?.code === "version_conflict") {
        setConflict(true);
        setError("该配置已被其他操作更新，请重新加载后再保存。");
      } else {
        setError("保存或验证失败，请检查输入或连接后重试。");
      }
    } finally {
      setSaving(false);
    }
  };

  const handleDiscover = async () => {
    if (!editing || !providerState || checking) return;
    setChecking(true);
    setError(undefined);
    try {
      const id = providerState.id, version = providerState.version, key = crypto.randomUUID();
      const res = await discoverLlmProviderModels(id, version, key);
      setDiscoveredModels(res.items);
      const latest = await getLlmProvider(id);
      setProviderState({ ...latest, modelCount: res.items.length });
      await onSaved();
      setError("已获取最新可用模型列表。");
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setConflict(api?.status === 409 || api?.code === "version_conflict");
      setError(api?.status === 409 ? "配置已变化，请重新加载后再操作。" : "获取模型列表失败，请稍后重试。");
    } finally {
      setChecking(false);
    }
  };

  const handleVerifyManualModel = async () => {
    if (!editing || !providerState || checking) return;
    if (!manualModel.trim()) {
      setError("请输入要校验的模型名称。");
      return;
    }
    setChecking(true);
    setError(undefined);
    setManualValidationResult(null);
    try {
      const id = providerState.id, version = providerState.version, key = crypto.randomUUID();
      const res = await verifyLlmProvider(id, version, key, manualModel.trim());
      const latest = await getLlmProvider(id);
      setProviderState({ ...latest, modelCount: providerState.modelCount });
      await onSaved();
      if (res.validationStatus === "verified") {
        setManualValidationResult(`模型 ${manualModel.trim()} 校验成功！`);
      } else {
        setManualValidationResult(`模型 ${manualModel.trim()} 校验失败：${res.safeError?.message || "校验不通过"}`);
      }
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setError(api?.message || "校验模型失败，请检查输入或连接。");
    } finally {
      setChecking(false);
    }
  };

  const command = async (kind: "verify" | "discover" | "toggle") => {
    if (!editing || !providerState || checking) return;
    setChecking(true);
    setError(undefined);
    try {
      const id = providerState.id, version = providerState.version, key = crypto.randomUUID();
      if (kind === "verify") await verifyLlmProvider(id, version, key);
      else if (kind === "discover") await discoverLlmProviderModels(id, version, key);
      else await setLlmProviderEnabled(id, version, !providerState.enabled, key);
      
      const latest = await getLlmProvider(id);
      setProviderState({ ...latest, modelCount: providerState.modelCount });
      await onSaved();
      setError(kind === "verify" ? "验证已提交，请查看最新验证状态。" : "操作已完成，已刷新最新状态。");
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setConflict(api?.status === 409 || api?.code === "version_conflict");
      setError(api?.status === 409 ? "配置已变化，请重新加载后再操作。" : "操作未完成，请稍后重试。");
    } finally {
      setChecking(false);
    }
  };

  const filteredModels = discoveredModels
    .filter(m => m.available)
    .filter(m => m.displayName.toLowerCase().includes(modelSearch.toLowerCase()) || m.modelName.toLowerCase().includes(modelSearch.toLowerCase()));

  if (editing && providerState) {
    return (
      <div className="llm-drawer-backdrop">
        <aside className="llm-drawer ui002-provider-drawer" role="dialog" aria-modal="true">
          <header>
            <div>
              <h2>编辑 LLM 配置</h2>
              <p>关键连接参数修改后将失去执行资格，重新验证成功后恢复。</p>
            </div>
            <button type="button" aria-label="关闭" onClick={onClose} disabled={saving || checking}>×</button>
          </header>
          <form onSubmit={submit}>
            <div className="llm-drawer-body">
              {/* 1. 基本信息 */}
              <div className="ui002-drawer-section">
                <h3>基本信息</h3>
                <div className="ui002-section-content">
                  <label>配置名称 *
                    <input
                      value={form.name}
                      maxLength={120}
                      onChange={event => set("name", event.target.value)}
                      disabled={saving || checking}
                    />
                  </label>
                  <label>服务类型 *
                    <select
                      value={form.providerType}
                      onChange={event => set("providerType", event.target.value as LlmProviderType)}
                      disabled={true}
                    >
                      {types.map(type => <option key={type.providerType} value={type.providerType}>{type.displayName}</option>)}
                    </select>
                    <small>服务类型创建后不可修改。</small>
                  </label>
                  <label>服务地址 *
                    <input
                      type="url"
                      value={form.baseUrl}
                      maxLength={512}
                      onChange={event => set("baseUrl", event.target.value)}
                      disabled={saving || checking}
                    />
                  </label>
                  <label>请求超时（秒）
                    <input
                      type="number"
                      min="5"
                      max="300"
                      value={form.timeoutSeconds}
                      onChange={event => set("timeoutSeconds", Number(event.target.value))}
                      disabled={saving || checking}
                    />
                  </label>
                </div>
              </div>

              {/* 2. 凭据 */}
              <div className="ui002-drawer-section">
                <h3>凭据</h3>
                <div className="ui002-section-content">
                  {providerState.hasSecret && !showSecretInput ? (
                    <div className="ui002-credential-status">
                      <span className="ui002-credential-text">API Key：已安全保存</span>
                      <button
                        type="button"
                        onClick={() => setShowSecretInput(true)}
                        disabled={saving || checking}
                      >
                        更新 API Key
                      </button>
                    </div>
                  ) : (
                    <label>API Key {(!providerState.hasSecret) ? " *" : ""}
                      <input
                        type="password"
                        value={form.secret}
                        autoComplete="new-password"
                        placeholder={providerState.hasSecret ? "输入新的 API Key" : "输入 API Key"}
                        onChange={event => set("secret", event.target.value)}
                        disabled={saving || checking}
                      />
                      <small>
                        {providerState.hasSecret ? "留空表示保留原凭据。" : "密钥加密保存，保存后不会回显明文。"}
                      </small>
                    </label>
                  )}
                </div>
              </div>

              {/* 3. 模型配置 */}
              <div className="ui002-drawer-section ui002-model-section">
                <h3>模型配置</h3>
                <div className="ui002-section-content">
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                    <span style={{ fontWeight: 600, fontSize: "14px", color: "#39485a" }}>可用模型列表</span>
                    <button
                      type="button"
                      onClick={handleDiscover}
                      disabled={saving || checking}
                      className="ui002-discover-btn"
                    >
                      获取模型列表
                    </button>
                  </div>

                  <div className="ui002-model-search-wrap">
                    <input
                      type="text"
                      placeholder="搜索可用模型名称..."
                      value={modelSearch}
                      onChange={e => setModelSearch(e.target.value)}
                      disabled={saving || checking}
                      className="ui002-model-search-input"
                    />
                  </div>

                  {discoveredModels.length > 0 ? (
                    <div className="ui002-model-list" style={{ maxHeight: "150px", overflowY: "auto", border: "1px solid #e2e8f0", borderRadius: "6px", padding: "8px" }}>
                      {filteredModels.length > 0 ? (
                        filteredModels.map(m => (
                          <div
                            key={m.modelName}
                            className={`ui002-model-item ${form.defaultModel === m.modelName ? "active" : ""}`}
                            onClick={() => set("defaultModel", m.modelName)}
                            style={{
                              padding: "6px 8px",
                              borderRadius: "4px",
                              marginBottom: "4px",
                              cursor: "pointer",
                              display: "flex",
                              justifyContent: "space-between",
                              alignItems: "center",
                              backgroundColor: form.defaultModel === m.modelName ? "#eff6ff" : "transparent"
                            }}
                          >
                            <span style={{ fontSize: "13px" }}>{m.displayName} ({m.modelName})</span>
                            {form.defaultModel === m.modelName && <span style={{ color: "#2563eb", fontSize: "11px", fontWeight: "bold" }}>默认</span>}
                          </div>
                        ))
                      ) : (
                        <div style={{ fontSize: "12px", color: "#64748b", textAlign: "center", padding: "8px" }}>未找到匹配的模型</div>
                      )}
                    </div>
                  ) : (
                    <div className="ui002-model-empty-state" style={{ padding: "12px", backgroundColor: "#f8fafc", borderRadius: "8px", border: "1px dashed #cbd5e1" }}>
                      <div style={{ fontSize: "13px", color: "#334155", marginBottom: "4px" }}>当前已知默认模型: <strong>{form.defaultModel}</strong></div>
                      <div style={{ fontSize: "13px", color: "#334155", marginBottom: "4px" }}>已发现模型数量: <strong>{providerState.modelCount ?? 0}</strong></div>
                      <div style={{ color: "#64748b", fontSize: "12px" }}>点击上方“获取模型列表”以加载可用模型。</div>
                    </div>
                  )}

                  <label>默认模型 *
                    <input
                      value={form.defaultModel}
                      maxLength={160}
                      onChange={event => set("defaultModel", event.target.value)}
                      disabled={saving || checking}
                    />
                  </label>

                  <div style={{ display: "flex", gap: "8px", alignItems: "flex-end" }}>
                    <label style={{ flex: 1 }}>手动输入模型
                      <input
                        type="text"
                        placeholder="输入待验证的模型名称"
                        value={manualModel}
                        onChange={e => setManualModel(e.target.value)}
                        disabled={saving || checking}
                      />
                    </label>
                    <button
                      type="button"
                      onClick={handleVerifyManualModel}
                      disabled={saving || checking}
                      style={{ height: "38px", padding: "0 16px", marginBottom: "2px" }}
                    >
                      校验模型
                    </button>
                  </div>
                  {manualValidationResult && (
                    <div style={{ fontSize: "12px", color: manualValidationResult.includes("成功") ? "#16a34a" : "#dc2626", marginTop: "4px" }}>
                      {manualValidationResult}
                    </div>
                  )}
                </div>
              </div>

              {/* 4. 验证与状态 */}
              <div className="ui002-drawer-section ui002-validation-section">
                <h3>验证与状态</h3>
                <div className="ui002-section-content">
                  <div className="ui002-status-grid">
                    <div className="ui002-status-item">
                      <span className="ui002-status-label">验证状态</span>
                      <span className={`ui002-status-badge validation-status-${providerState.validationStatus}`}>
                        {providerState.validationStatus === "verified" ? "验证成功" :
                         providerState.validationStatus === "failed" ? "验证失败" :
                         providerState.validationStatus === "stale" ? "配置已变更" :
                         providerState.validationStatus === "verifying" ? "验证中" : "未验证"}
                      </span>
                    </div>
                    <div className="ui002-status-item">
                      <span className="ui002-status-label">启用状态</span>
                      <span className={`ui002-status-badge enabled-status-${providerState.enabled}`}>
                        {(providerState.validationStatus === "stale" && providerState.enabled && providerState.executable === false)
                          ? "启用状态已保留"
                          : (providerState.enabled ? "已启用" : "未启用")}
                      </span>
                    </div>
                    <div className="ui002-status-item">
                      <span className="ui002-status-label">执行资格</span>
                      <span className={`ui002-status-badge executable-status-${providerState.executable}`}>
                        {providerState.executable ? "可执行" : "不可执行"}
                      </span>
                    </div>
                  </div>

                  {providerState.validationStatus === "stale" && providerState.enabled && providerState.executable === false && (
                    <div className="ui002-impact-warning">
                      <p className="ui002-impact-biz">
                        Base URL、API Key、超时或模型配置变化会使验证失效，但不会自动停用或解除项目绑定。
                      </p>
                      <p className="ui002-impact-rel">
                        现有工作流配置和项目绑定关系保持不变；重新验证成功前，相关新运行暂时被阻断。
                      </p>
                    </div>
                  )}

                  <div className="ui002-status-times">
                    <div>最近验证时间：{formatDateTime(providerState.lastVerifiedAt)}</div>
                    {providerState.checkedAt && <div>检查时间：{formatDateTime(providerState.checkedAt)}</div>}
                  </div>

                  {(providerState.lastErrorMessage || providerState.safeError) && (
                    <div className="ui002-status-error-msg">
                      {providerState.lastErrorMessage || providerState.safeError?.message || "未知验证错误"}
                    </div>
                  )}

                  <div className="ui002-validation-actions">
                    <button
                      type="button"
                      onClick={() => void command("verify")}
                      disabled={saving || checking}
                    >
                      验证配置
                    </button>
                    <button
                      type="button"
                      onClick={() => void command("toggle")}
                      disabled={saving || checking}
                    >
                      {providerState.enabled ? "停用配置" : "启用配置"}
                    </button>
                  </div>
                </div>
              </div>

              {error && (
                <div className="llm-form-error" role="alert">
                  {error}
                  {conflict && (
                    <button type="button" onClick={() => void reload()} disabled={saving || checking}>
                      重新加载
                    </button>
                  )}
                </div>
              )}
            </div>
            <footer className="ui002-drawer-footer">
              <button type="button" onClick={onClose} disabled={saving || checking}>
                取消
              </button>
              <div className="ui002-footer-right">
                <button
                  type="submit"
                  disabled={saving || checking}
                >
                  保存
                </button>
                <button
                  type="button"
                  className="primary"
                  onClick={handleSaveAndVerify}
                  disabled={saving || checking}
                >
                  {saving ? "保存中…" : "保存并验证"}
                </button>
              </div>
            </footer>
          </form>
        </aside>
      </div>
    );
  }

  return <div className="llm-drawer-backdrop"><aside className="llm-drawer" role="dialog" aria-modal="true"><header><div><h2>{editing ? "编辑 LLM 配置" : "添加 LLM 配置"}</h2><p>配置全局 LLM 连接，供项目工作流调用</p></div><button type="button" aria-label="关闭" onClick={onClose} disabled={saving || checking}>×</button></header><form onSubmit={submit}><div className="llm-drawer-body"><label>配置名称 *<input value={form.name} maxLength={120} onChange={event => set("name", event.target.value)} disabled={saving || checking} /></label><label>服务类型 *<select value={form.providerType} onChange={event => set("providerType", event.target.value as LlmProviderType)} disabled={editing || saving || checking}>{types.map(type => <option key={type.providerType} value={type.providerType}>{type.displayName}</option>)}</select>{editing && <small>服务类型创建后不可修改。</small>}</label><label>服务地址 *<input type="url" value={form.baseUrl} maxLength={512} onChange={event => set("baseUrl", event.target.value)} disabled={saving || checking} /></label><label>默认模型 *<input value={form.defaultModel} maxLength={160} onChange={event => set("defaultModel", event.target.value)} disabled={saving || checking} /></label>{secretSupported && <label>API Key{editing ? "" : " *"}<input type="password" value={form.secret} autoComplete="new-password" onChange={event => set("secret", event.target.value)} disabled={saving || checking} /><small>{editing && drawer.provider.hasSecret ? "已配置；留空将保留原密钥。" : "密钥加密保存，保存后不会回显明文。"}</small></label>}<label>请求超时（秒）<input type="number" min="5" max="300" value={form.timeoutSeconds} onChange={event => set("timeoutSeconds", Number(event.target.value))} disabled={saving || checking} /></label>{editing && <div className="llm-deferred"><button type="button" onClick={() => void command("verify")} disabled={saving || checking}>验证配置</button><button type="button" onClick={() => void command("discover")} disabled={saving || checking}>发现模型</button><button type="button" onClick={() => void command("toggle")} disabled={saving || checking}>{drawer.provider.enabled ? "停用配置" : "启用配置"}</button><p>保存、验证与启停是独立操作；密钥始终不会回显。</p></div>}{error && <div className="llm-form-error" role="alert">{error}{conflict && <button type="button" onClick={() => void reload()} disabled={saving || checking}>重新加载</button>}</div>}</div><footer><button type="button" onClick={onClose} disabled={saving || checking}>取消</button><button className="primary" disabled={saving || checking}>{saving ? "保存中…" : "保存"}</button></footer></form></aside></div>;
}

function formatDateTime(isoString: string | null): string {
  if (!isoString) return "—";
  try {
    const d = new Date(isoString);
    if (isNaN(d.getTime())) return "—";
    const pad = (n: number) => String(n).padStart(2, '0');
    const year = d.getFullYear();
    const month = pad(d.getMonth() + 1);
    const day = pad(d.getDate());
    const hours = pad(d.getHours());
    const minutes = pad(d.getMinutes());
    return `${year}-${month}-${day} ${hours}:${minutes}`;
  } catch {
    return "—";
  }
}

export function SettingsPage() {
  const [providers, setProviders] = useState<LlmProviderVm[] | null>(null);
  const [types, setTypes] = useState<LlmProviderTypeDto[] | null>(null);
  const [total, setTotal] = useState(0);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("");
  const [enabled, setEnabled] = useState("");
  const [executable, setExecutable] = useState("");
  const [offset, setOffset] = useState(0);
  const [drawer, setDrawer] = useState<Drawer>(null);
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [verifyingId, setVerifyingId] = useState<string | null>(null);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  
  const limit = 20;

  const load = useCallback(async (signal?: AbortSignal) => {
    setError(undefined);
    setLoading(true);
    try {
      const [result, catalogue] = await Promise.all([
        listLlmProviders({
          q: query || undefined,
          validationStatus: (status || undefined) as ValidationStatus | undefined,
          enabled: enabled === "true" ? true : enabled === "false" ? false : undefined,
          executable: executable === "true" ? true : executable === "false" ? false : undefined,
          limit,
          offset
        }, { signal }),
        listLlmProviderTypes({ signal })
      ]);
      setProviders(result.items.map(mapLlmProvider));
      setTypes(catalogue.items);
      setTotal(result.total);
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.code === "cancelled")) {
        setError("暂时无法加载 LLM 配置，请稍后重试。");
      }
    } finally {
      setLoading(false);
    }
  }, [enabled, executable, offset, query, status]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => void load(controller.signal), 300);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [load]);

  useEffect(() => {
    const handleOutsideClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (!target.closest('.ui001-more-menu-container')) {
        setOpenMenuId(null);
      }
    };
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpenMenuId(null);
      }
    };
    document.addEventListener("click", handleOutsideClick);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("click", handleOutsideClick);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

  const reset = (action: () => void) => {
    setOffset(0);
    action();
  };

  const edit = async (provider: LlmProviderVm) => {
    try {
      const detail = await getLlmProvider(provider.id);
      setDrawer({ mode: "edit", provider: { ...detail, modelCount: provider.modelCount } });
    } catch {
      setNotice("暂时无法读取该配置详情。");
    }
  };

  const handleToggleEnabled = async (provider: LlmProviderVm) => {
    if (togglingId) return;
    setTogglingId(provider.id);
    setNotice(undefined);
    try {
      await setLlmProviderEnabled(provider.id, provider.version, !provider.enabled, crypto.randomUUID());
      await load();
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setError(api?.message || "操作失败，请稍后重试。");
    } finally {
      setTogglingId(null);
    }
  };

  const handleVerify = async (provider: LlmProviderVm) => {
    if (verifyingId) return;
    setVerifyingId(provider.id);
    setNotice(undefined);
    try {
      await verifyLlmProvider(provider.id, provider.version, crypto.randomUUID());
      await load();
      setNotice("验证成功。");
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setError(api?.message || "验证失败，请重试。");
    } finally {
      setVerifyingId(null);
    }
  };

  const handleDiscoverModels = async (provider: LlmProviderVm) => {
    setOpenMenuId(null);
    setTogglingId(provider.id);
    setNotice(undefined);
    try {
      await discoverLlmProviderModels(provider.id, provider.version, crypto.randomUUID());
      await load();
      setNotice("发现模型。");
    } catch (cause) {
      const api = cause instanceof ApiError ? cause : undefined;
      setError(api?.message || "模型发现失败，请重试。");
    } finally {
      setTogglingId(null);
    }
  };

  const page = Math.floor(offset / limit) + 1;
  const pages = Math.max(1, Math.ceil(total / limit));

  return (
    <section className="settings-content ui001-provider-list">
      <div className="ui001-header">
        <button className="primary" onClick={() => setDrawer({ mode: "create" })} disabled={!types?.length}>
          添加 LLM 配置
        </button>
      </div>

      {/* 4.3 仅在有配置列表时展示顶部规则说明条 */}
      {providers && providers.length > 0 && (
        <div className="ui001-provider-notice">
          关键连接参数修改后，当前启用状态会保留，但执行资格立即失效。重新验证成功后才能恢复执行。
        </div>
      )}

      {notice && <p className="llm-toast" role="status">{notice}</p>}

      {error ? (
        <div className="llm-state" role="alert">
          <h3>暂时无法加载</h3>
          <p>{error}</p>
          <button onClick={() => void load()}>重试</button>
        </div>
      ) : !providers || !types ? (
        <div className="llm-loading" role="status">正在加载 LLM 配置…</div>
      ) : (total === 0 && !query && !status && !enabled && !executable) ? (
        <div className="ui003-empty-container">
          <div className="ui003-empty-card">
            <div className="ui003-empty-icon">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
                <rect x="3" y="11" width="18" height="10" rx="2"/>
                <circle cx="12" cy="5" r="2"/>
                <path d="M12 7v4"/>
                <line x1="8" y1="16" x2="8.01" y2="16"/>
                <line x1="16" y1="16" x2="16.01" y2="16"/>
              </svg>
            </div>
            <h3 className="ui003-empty-title">暂无 LLM 配置</h3>
            <p className="ui003-empty-desc">添加全局 LLM 配置，供项目工作流调用</p>
            <button
              className="primary ui003-empty-cta"
              onClick={() => setDrawer({ mode: "create" })}
              disabled={!types?.length}
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="12" y1="5" x2="12" y2="19"/>
                <line x1="6" y1="12" x2="18" y2="12"/>
              </svg>
              添加 LLM 配置
            </button>
            <div className="ui003-empty-tip">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
                <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
              </svg>
              <span>支持 OpenAI-compatible Provider；密钥保存后不会回显明文</span>
            </div>
          </div>
        </div>
      ) : (
        <>
          <div className="ui001-toolbar">
            {/* 1. 搜索框 */}
            <input
              value={query}
              onChange={event => reset(() => setQuery(event.target.value))}
              placeholder="搜索配置名称"
            />

            {/* 2. 验证状态 */}
            <select value={status} onChange={event => reset(() => setStatus(event.target.value))}>
              <option value="">全部验证状态</option>
              <option value="unverified">未验证</option>
              <option value="verifying">验证中</option>
              <option value="verified">验证成功</option>
              <option value="failed">验证失败</option>
              <option value="stale">配置已变更</option>
            </select>

            {/* 3. 启用状态 */}
            <select value={enabled} onChange={event => reset(() => setEnabled(event.target.value))}>
              <option value="">全部启用状态</option>
              <option value="true">已启用</option>
              <option value="false">未启用</option>
            </select>

            {/* 4. 执行资格 */}
            <select value={executable} onChange={event => reset(() => setExecutable(event.target.value))}>
              <option value="">全部执行资格</option>
              <option value="true">可执行</option>
              <option value="false">不可执行</option>
            </select>

            {/* 5. 刷新按钮 */}
            <button
              type="button"
              className="ui001-refresh-btn"
              onClick={() => void load()}
              disabled={loading}
            >
              刷新
            </button>

            {/* 6. 配置总数 */}
            <span className="ui001-total-count">共 {total} 个配置</span>
          </div>

          <div className="ui001-table-wrap">
            <table className="ui001-table">
              <thead>
                <tr>
                  <th>配置名称</th>
                  <th>服务类型</th>
                  <th>Base URL</th>
                  <th>可用模型</th>
                  <th>默认模型</th>
                  <th>验证状态</th>
                  <th>启用状态</th>
                  <th>执行资格</th>
                  <th>最近验证</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {providers.map(provider => {
                  const rowDisabled = togglingId === provider.id || verifyingId === provider.id;
                  return (
                    <tr key={provider.id}>
                      <td>
                        <strong>{provider.name}</strong>
                        {provider.hasSecret && (
                          <small>
                            凭据已配置
                            {provider.secretFingerprint && ` · ${provider.secretFingerprint}`}
                          </small>
                        )}
                      </td>
                      <td>{provider.providerTypeLabel}</td>
                      <td title={provider.baseUrl} className="ui001-base-url">
                        {provider.baseUrl}
                      </td>
                      <td>{provider.modelCount} 个</td>
                      <td>
                        {provider.defaultModel ? (
                          <span className="ui001-default-model-badge">
                            {provider.defaultModel}
                          </span>
                        ) : "—"}
                      </td>
                      <td>
                        <div className="ui001-status-container">
                          <span className={`ui001-status ui001-status-${provider.validationStatus}`}>
                            {provider.validationStatusLabel}
                          </span>
                          {provider.validationStatus === "failed" && (provider.lastErrorMessage || provider.safeError) && (
                            <div className="ui001-status-error" title={provider.lastErrorMessage || provider.safeError || ""}>
                              {provider.lastErrorMessage || provider.safeError}
                            </div>
                          )}
                          {provider.validationStatus === "stale" && (
                            <div className="ui001-status-error">
                              原验证结果已失效，请重新验证
                            </div>
                          )}
                        </div>
                      </td>
                      <td>
                        <button
                          role="switch"
                          aria-checked={provider.enabled}
                          disabled={rowDisabled}
                          onClick={() => void handleToggleEnabled(provider)}
                          className={`ui001-toggle-btn ${provider.enabled ? "enabled" : "disabled"}`}
                        />
                      </td>
                      <td>
                        {provider.executable ? (
                          <span className="ui001-eligibility-ready">可执行</span>
                        ) : (
                          <span className="ui001-eligibility-blocked" title={provider.safeError || "不可执行"}>
                            不可执行
                          </span>
                        )}
                      </td>
                      <td>
                        {formatDateTime(provider.checkedAt ?? provider.lastVerifiedAt)}
                      </td>
                      <td className="ui001-actions">
                        <button className="ui001-action-btn" disabled={rowDisabled} onClick={() => void edit(provider)}>
                          编辑
                        </button>
                        <button className="ui001-action-btn" disabled={rowDisabled} onClick={() => void handleVerify(provider)}>
                          {provider.validationStatus === "stale" ? "重新验证" : "验证"}
                        </button>

                        <div className="ui001-more-menu-container">
                          <button
                            type="button"
                            className="ui001-more-btn ui001-action-btn"
                            disabled={rowDisabled}
                            onClick={(e) => {
                              e.stopPropagation();
                              setOpenMenuId(openMenuId === provider.id ? null : provider.id);
                            }}
                          >
                            更多
                          </button>
                          {openMenuId === provider.id && (
                            <ul className="ui001-more-menu" role="menu">
                              <li role="none">
                                <button
                                  type="button"
                                  role="menuitem"
                                  onClick={() => void handleDiscoverModels(provider)}
                                >
                                  发现模型
                                </button>
                              </li>
                              <li role="none">
                                <button
                                  type="button"
                                  role="menuitem"
                                  onClick={() => void handleToggleEnabled(provider)}
                                >
                                  {provider.enabled ? "停用配置" : "启用配置"}
                                </button>
                              </li>
                            </ul>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="ui001-pagination">
            <span>第 {page} / {pages} 页</span>
            <button
              disabled={offset === 0}
              onClick={() => setOffset(value => Math.max(0, value - limit))}
            >
              上一页
            </button>
            <button
              disabled={offset + limit >= total}
              onClick={() => setOffset(value => value + limit)}
            >
              下一页
            </button>
          </div>
        </>
      )}

      {drawer && types && (
        <ProviderDrawer
          key={drawer.mode === "edit" ? drawer.provider.id : "create"}
          drawer={drawer}
          types={types}
          onClose={() => setDrawer(null)}
          onSaved={async () => {
            await load();
            setNotice("配置已保存，并已同步服务端最新数据。");
          }}
        />
      )}
    </section>
  );
}
