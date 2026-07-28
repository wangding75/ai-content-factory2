from __future__ import annotations

import hashlib
import shutil
from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
ITERATION = (
    ROOT
    / "docs"
    / "development-inputs"
    / "p1"
    / "iterations"
    / "iteration-15-real-chapter-planning"
)
EVIDENCE = ITERATION / "evidence"
FRAME_ROOT = ITERATION / "ui" / "frames"

FRAMES = [
    ("01", "running-mainline-expansion", "P15_C1_RUNNING_MAINLINE_EXPANSION"),
    ("02", "running-partial-range", "P15_C1_RUNNING_PARTIAL_RANGE"),
    ("03", "running-full-plan", "P15_C1_RUNNING_FULL_PLAN"),
    ("04", "running-partial-eta", "P15_C1_RUNNING_PARTIAL_ETA"),
    ("05", "running-partial-validating", "P15_C1_RUNNING_PARTIAL_VALIDATING"),
    ("06", "running-data-variant", "P15_C1_RUNNING_DATA_VARIANT"),
    ("07", "failed-atomic", "P15_C1_FAILED_ATOMIC"),
    ("08", "not-configured", "P15_C1_NOT_CONFIGURED"),
    ("09", "generation-settings", "P15_C2_GENERATION_SETTINGS"),
    ("10", "preflight-progress", "P15_C2_PREFLIGHT_PROGRESS"),
    ("11", "preflight-pass", "P15_C3_PREFLIGHT_PASS"),
    ("12", "preflight-blocked", "P15_C3_PREFLIGHT_BLOCKED"),
    ("13", "run-created", "P15_C3_RUN_CREATED"),
    ("14", "candidate-batch-list", "P15_C4_CANDIDATE_BATCH_LIST"),
    ("15", "candidate-batch-detail", "P15_C5_CANDIDATE_BATCH_DETAIL"),
    ("16", "candidate-edit-drawer", "P15_C6_CANDIDATE_EDIT_DRAWER"),
    ("17", "candidate-compare-dialog", "P15_C7_CANDIDATE_COMPARE_DIALOG"),
    ("18", "batch-adopt-dialog", "P15_C8_BATCH_ADOPT_DIALOG"),
    ("19", "batch-abandon-dialog", "P15_C9_BATCH_ABANDON_DIALOG"),
    ("20", "stale-conflict-dialog", "P15_C10_STALE_CONFLICT_DIALOG"),
    ("21", "storyline-relation-readonly", "P15_S1_STORYLINE_RELATION_READONLY"),
]


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def create_comparison(prototype_img: Image.Image, actual_img: Image.Image) -> Image.Image:
    w, h = prototype_img.size
    act = actual_img.convert("RGB")
    if act.size != (w, h):
        act = act.resize((w, h), Image.Resampling.LANCZOS)
    canvas = Image.new("RGB", (w * 2, h), "#e5e7eb")
    canvas.paste(prototype_img.convert("RGB"), (0, 0))
    canvas.paste(act, (w, 0))
    return canvas


def create_overlay(prototype_img: Image.Image, actual_img: Image.Image) -> Image.Image:
    p = prototype_img.convert("RGBA")
    a = actual_img.convert("RGBA")
    if a.size != p.size:
        a = a.resize(p.size, Image.Resampling.LANCZOS)
    return Image.blend(p, a, alpha=0.5)


def main() -> None:
    EVIDENCE.mkdir(parents=True, exist_ok=True)
    actual_digests: dict[str, Path] = {}

    for number, slug, frame in FRAMES:
        source = FRAME_ROOT / frame / "screen.png"
        prototype = EVIDENCE / f"{number}-{slug}-prototype.png"
        actual = EVIDENCE / f"{number}-{slug}-actual.png"
        comparison = EVIDENCE / f"{number}-{slug}-comparison.png"
        overlay = EVIDENCE / f"{number}-{slug}-overlay.png"

        if not source.is_file():
            raise FileNotFoundError(f"missing frozen prototype: {source}")
        if not actual.is_file():
            raise FileNotFoundError(f"missing Chromium screenshot: {actual}")

        shutil.copyfile(source, prototype)

        actual_digest = digest(actual)
        if actual_digest in actual_digests:
            raise ValueError(
                f"duplicate actual screenshots: {actual_digests[actual_digest]} and {actual}"
            )
        actual_digests[actual_digest] = actual

        with Image.open(source) as prototype_image, Image.open(actual) as actual_image:
            comp_img = create_comparison(prototype_image, actual_image)
            comp_img.save(comparison, "PNG", optimize=True)

            overlay_img = create_overlay(prototype_image, actual_image)
            overlay_img.save(overlay, "PNG", optimize=True)

    prototypes = list(EVIDENCE.glob("*-prototype.png"))
    actuals = list(EVIDENCE.glob("*-actual.png"))
    comparisons = list(EVIDENCE.glob("*-comparison.png"))
    overlays = list(EVIDENCE.glob("*-overlay.png"))

    if (len(prototypes), len(actuals), len(comparisons)) != (21, 21, 21):
        raise RuntimeError(
            "evidence count mismatch: "
            f"prototype={len(prototypes)} actual={len(actuals)} "
            f"comparison={len(comparisons)}"
        )
    print("STATUS: PASS")
    print(f"prototype={len(prototypes)} actual={len(actuals)} comparison={len(comparisons)} overlay={len(overlays)}")


if __name__ == "__main__":
    main()
