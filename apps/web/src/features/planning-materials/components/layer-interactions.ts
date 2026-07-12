"use client";

import {useEffect, useRef, type RefObject} from "react";

type LayerEntry = {id: number; onEscape: () => void};

export function createLayerStack() {
  const entries: LayerEntry[] = [];
  return {
    add(entry: LayerEntry) { entries.push(entry); },
    remove(id: number) { const index = entries.findIndex((entry) => entry.id === id); if (index >= 0) entries.splice(index, 1); },
    isTop(id: number) { return entries.at(-1)?.id === id; },
    size() { return entries.length; },
  };
}

const layerStack = createLayerStack();
let layerId = 0;
let savedBodyOverflow: string | null = null;

export function nextLayerFocusIndex(length: number, currentIndex: number, shiftKey: boolean) {
  if (length === 0) return -1;
  if (currentIndex < 0) return shiftKey ? length - 1 : 0;
  if (shiftKey && currentIndex === 0) return length - 1;
  if (!shiftKey && currentIndex === length - 1) return 0;
  return -1;
}

function focusable(root: HTMLElement) {
  return Array.from(root.querySelectorAll<HTMLElement>('button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')).filter((element) => !element.hidden && element.getAttribute("aria-hidden") !== "true");
}

export function useLayerInteractions<T extends HTMLElement = HTMLElement>(onEscape: () => void, initialFocus?: RefObject<HTMLElement | null>) {
  const rootRef = useRef<T | null>(null);
  const idRef = useRef<number | null>(null);

  useEffect(() => {
    const id = ++layerId;
    idRef.current = id;
    const restoreFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    layerStack.add({id, onEscape});
    if (layerStack.size() === 1) {
      savedBodyOverflow = document.body.style.overflow;
      document.body.style.overflow = "hidden";
    }

    const focusLayer = () => {
      const target = initialFocus?.current ?? rootRef.current?.querySelector<HTMLElement>("[data-layer-initial-focus]") ?? rootRef.current;
      target?.focus();
    };
    const frame = window.requestAnimationFrame(focusLayer);
    const onKeyDown = (event: KeyboardEvent) => {
      if (!layerStack.isTop(id) || event.isComposing) return;
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        onEscape();
        return;
      }
      if (event.key !== "Tab") return;
      const root = rootRef.current;
      if (!root) return;
      const items = focusable(root);
      if (!items.length) {
        event.preventDefault();
        root.focus();
        return;
      }
      const currentIndex = items.indexOf(document.activeElement as HTMLElement);
      const destination = nextLayerFocusIndex(items.length, currentIndex, event.shiftKey);
      if (destination >= 0) {
        event.preventDefault();
        items[destination].focus();
      }
    };
    window.addEventListener("keydown", onKeyDown, true);
    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener("keydown", onKeyDown, true);
      layerStack.remove(id);
      if (layerStack.size() === 0) {
        document.body.style.overflow = savedBodyOverflow ?? "";
        savedBodyOverflow = null;
      }
      if (restoreFocus?.isConnected) restoreFocus.focus();
    };
  }, [initialFocus, onEscape]);

  return rootRef;
}
