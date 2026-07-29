"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { Icon } from "@/components/ui/icons";
import type { ContentVersion } from "@/features/content-items/content-item-http-api";
import {
  createContentReviewRun,
  getReviewSourceVersion,
  listReviewableContentVersions,
  preflightContentReview,
  type ReviewPreflightReport,
  type ReviewableContentVersion,
  type WorkflowRunDto,
} from "./content-review-api";
import { reviewCopy as copy } from "./content-review-locale";
import {
  formatReviewTime,
  reviewDimensionLabel,
  safeReviewError,
} from "./content-review-presentation";

export function ContentReviewDrawer({
  projectId,
  version,
  onClose,
  onCreated,
}: {
  projectId: string;
  version: ContentVersion;
  onClose: () => void;
  onCreated: (run: WorkflowRunDto) => void;
}) {
  const [instructions, setInstructions] = useState("");
  const [preflight, setPreflight] = useState<ReviewPreflightReport | null>(
    null,
  );
  const [checking, setChecking] = useState(false);
  const [creating, setCreating] = useState(false);
  const [versionsLoading, setVersionsLoading] = useState(true);
  const [versionLoading, setVersionLoading] = useState(false);
  const [versionsError, setVersionsError] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [versions, setVersions] = useState<ReviewableContentVersion[]>([
    {
      id: version.id,
      content_item_id: version.content_item_id,
      version_no: version.version_no,
      status: version.status,
      title: version.title,
      word_count: version.word_count,
      created_at: version.created_at,
      frozen_at: version.frozen_at,
      is_current: true,
    },
  ]);
  const [selectedVersion, setSelectedVersion] = useState(version);
  const sequence = useRef(0);
  const createKey = useRef<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    void listReviewableContentVersions(version.content_item_id, {
      signal: controller.signal,
    })
      .then((result) => {
        if (controller.signal.aborted) return;
        const current = {
          id: version.id,
          content_item_id: version.content_item_id,
          version_no: version.version_no,
          status: version.status,
          title: version.title,
          word_count: version.word_count,
          created_at: version.created_at,
          frozen_at: version.frozen_at,
          is_current: true,
        };
        setVersions(
          result.items.some((item) => item.id === version.id)
            ? result.items
            : [current, ...result.items],
        );
      })
      .catch(() => {
        if (!controller.signal.aborted) setVersionsError(true);
      })
      .finally(() => {
        if (!controller.signal.aborted) setVersionsLoading(false);
      });
    return () => controller.abort();
  }, [version]);

  const selectVersion = async (item: ReviewableContentVersion) => {
    if (item.id === selectedVersion.id || versionLoading || creating) return;
    const request = ++sequence.current;
    setVersionLoading(true);
    setPreflight(null);
    setError(null);
    createKey.current = null;
    try {
      const result = await getReviewSourceVersion(item.id);
      if (request === sequence.current)
        setSelectedVersion(result.content_version);
    } catch (cause) {
      if (request === sequence.current)
        setError(safeReviewError(cause, copy.drawer.preflightFailed));
    } finally {
      if (request === sequence.current) setVersionLoading(false);
    }
  };

  const runPreflight = useCallback(
    async (value: string, signal?: AbortSignal) => {
      const request = ++sequence.current;
      setChecking(true);
      setPreflight(null);
      setError(null);
      createKey.current = null;
      try {
        const result = await preflightContentReview(
          selectedVersion.id,
          {
            sourceContentVersionVersion: selectedVersion.version,
            optionalInstructions: value.trim() || null,
          },
          { signal },
        );
        if (!signal?.aborted && request === sequence.current)
          setPreflight(result);
      } catch (cause) {
        if (!signal?.aborted && request === sequence.current)
          setError(safeReviewError(cause, copy.drawer.preflightFailed));
      } finally {
        if (!signal?.aborted && request === sequence.current)
          setChecking(false);
      }
    },
    [selectedVersion.id, selectedVersion.version],
  );

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(
      () => void runPreflight(instructions, controller.signal),
      instructions ? 350 : 0,
    );
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [instructions, runPreflight]);

  useEffect(
    () => () => {
      sequence.current += 1;
    },
    [],
  );

  const create = async () => {
    if (
      creating ||
      checking ||
      preflight?.status !== "passed" ||
      !preflight.preflightToken
    )
      return;
    setCreating(true);
    setError(null);
    const key = createKey.current ?? crypto.randomUUID();
    createKey.current = key;
    try {
      const run = await createContentReviewRun(
        selectedVersion.id,
        preflight.preflightToken,
        key,
      );
      createKey.current = null;
      onCreated(run);
    } catch (cause) {
      setError(safeReviewError(cause, copy.drawer.createFailed));
      setPreflight(null);
    } finally {
      setCreating(false);
    }
  };

  const notConfigured =
    error === copy.errors.notConfigured ||
    preflight?.checks.some(
      (check) =>
        check.status === "blocked" &&
        [
          "project_binding_available",
          "workflow_configuration_available",
          "workflow_connection_available",
        ].includes(check.code),
    );

  return (
    <div className="review-drawer-backdrop" onMouseDown={onClose}>
      <aside
        className="review-drawer"
        role="dialog"
        aria-modal="true"
        aria-labelledby="review-drawer-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header>
          <div>
            <h2 id="review-drawer-title">{copy.drawer.title}</h2>
            <p>{copy.drawer.subtitle}</p>
          </div>
          <button
            type="button"
            aria-label={copy.drawer.close}
            onClick={onClose}
            disabled={creating}
          >
            <Icon name="close" size={20} />
          </button>
        </header>
        <div className="review-drawer-scroll">
          <section className="review-drawer-source">
            <span>{copy.drawer.currentWork}</span>
            <h3>{version.title}</h3>
            <p>
              {copy.common.versionPrefix} V{version.version_no} ·{" "}
              {version.word_count.toLocaleString("zh-CN")} {copy.common.words} ·{" "}
              {copy.drawer.saved}
            </p>
          </section>

          <section>
            <h3>{copy.drawer.sourceVersion}</h3>
            <div className="review-version-list" role="radiogroup">
              {versions.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  role="radio"
                  aria-checked={item.id === selectedVersion.id}
                  className={`review-version-card${item.id === selectedVersion.id ? " selected" : ""}`}
                  onClick={() => void selectVersion(item)}
                  disabled={versionLoading || creating}
                >
                  <b>V{item.version_no}</b>
                  <span>
                    {item.word_count.toLocaleString("zh-CN")} {copy.common.words} ·{" "}
                    {formatReviewTime(item.created_at)} ·{" "}
                    {item.is_current
                      ? copy.drawer.currentVersion
                      : copy.drawer.historicalVersion}
                  </span>
                </button>
              ))}
            </div>
            {versionsLoading && (
              <p className="review-muted">{copy.drawer.versionsLoading}</p>
            )}
            {versionsError && (
              <p className="review-muted">{copy.drawer.versionsFailed}</p>
            )}
            {versionLoading && (
              <p className="review-muted">{copy.drawer.versionLoading}</p>
            )}
            <p className="review-helper">{copy.drawer.fixedHint}</p>
          </section>

          <section>
            <h3>{copy.drawer.workflow}</h3>
            {preflight?.configurationSummary ? (
              <div className="review-workflow-card">
                <Icon name="workflow" size={21} />
                <div>
                  <b>
                    {
                      preflight.configurationSummary
                        .workflowConfigurationName
                    }
                  </b>
                  <span>{copy.common.available}</span>
                </div>
                <Link
                  href={`/projects/${projectId}/settings?tab=workflow-bindings`}
                >
                  {copy.drawer.configuration}
                </Link>
              </div>
            ) : (
              <p className="review-muted">
                {checking ? copy.drawer.checking : copy.drawer.blocked}
              </p>
            )}
          </section>

          <section>
            <h3>{copy.drawer.dimensions}</h3>
            <div className="review-dimensions">
              {preflight?.reviewDimensions.map((dimension) => (
                <span key={dimension}>
                  {reviewDimensionLabel(dimension)}
                </span>
              ))}
            </div>
            <p className="review-helper">{copy.drawer.dimensionsHint}</p>
          </section>

          <label className="review-instructions">
            {copy.drawer.instructions}
            <textarea
              value={instructions}
              maxLength={2000}
              placeholder={copy.drawer.instructionsPlaceholder}
              onChange={(event) => {
                setInstructions(event.target.value);
                setPreflight(null);
                createKey.current = null;
              }}
              disabled={creating}
            />
            <small>{instructions.length}/2000</small>
          </label>

          <section className="review-preflight" aria-live="polite">
            <h3>{copy.drawer.preflight}</h3>
            {checking ? (
              <p className="checking">{copy.drawer.checking}</p>
            ) : preflight ? (
              <>
                <p className={preflight.status}>
                  {preflight.status === "passed"
                    ? copy.drawer.passed
                    : copy.drawer.blocked}
                </p>
                <ul>
                  {preflight.checks.map((check) => (
                    <li key={check.code} className={check.status}>
                      {check.message}
                    </li>
                  ))}
                </ul>
              </>
            ) : null}
            {error && (
              <p className="review-error" role="alert">
                {error}
              </p>
            )}
            {notConfigured && (
              <Link
                className="review-settings-link"
                href={`/projects/${projectId}/settings?tab=workflow-bindings`}
              >
                {copy.common.settings}
              </Link>
            )}
          </section>
        </div>
        <footer>
          <button type="button" onClick={onClose} disabled={creating}>
            {copy.drawer.cancel}
          </button>
          {error && !notConfigured && (
            <button
              type="button"
              onClick={() => void runPreflight(instructions)}
              disabled={checking || creating}
            >
              {copy.drawer.recheck}
            </button>
          )}
          <button
            type="button"
            className="primary"
            onClick={() => void create()}
            disabled={
              checking ||
              creating ||
              versionLoading ||
              preflight?.status !== "passed" ||
              !preflight.preflightToken
            }
          >
            {creating ? copy.drawer.creating : copy.drawer.confirm}
          </button>
        </footer>
      </aside>
    </div>
  );
}
