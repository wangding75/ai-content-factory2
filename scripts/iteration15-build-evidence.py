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
VIEWPORT = (1440, 1024)

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


def fit_on_canvas(image: Image.Image) -> Image.Image:
    output = Image.new("RGB", VIEWPORT, "white")
    copy = image.convert("RGB")
    copy.thumbnail(VIEWPORT, Image.Resampling.LANCZOS)
    offset = ((VIEWPORT[0] - copy.width) // 2, (VIEWPORT[1] - copy.height) // 2)
    output.paste(copy, offset)
    return output


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> None:
    EVIDENCE.mkdir(parents=True, exist_ok=True)
    actual_digests: dict[str, Path] = {}

    for number, slug, frame in FRAMES:
        source = FRAME_ROOT / frame / "screen.png"
        prototype = EVIDENCE / f"{number}-{slug}-prototype.png"
        actual = EVIDENCE / f"{number}-{slug}-actual.png"
        comparison = EVIDENCE / f"{number}-{slug}-comparison.png"

        if not source.is_file():
            raise FileNotFoundError(f"missing frozen prototype: {source}")
        if not actual.is_file():
            raise FileNotFoundError(f"missing Chromium screenshot: {actual}")

        with Image.open(actual) as actual_image:
            if actual_image.size != VIEWPORT:
                normalized_actual = fit_on_canvas(actual_image)
                normalized_actual.save(actual, "PNG", optimize=True)

        actual_digest = digest(actual)
        if actual_digest in actual_digests:
            raise ValueError(
                f"duplicate actual screenshots: {actual_digests[actual_digest]} and {actual}"
            )
        actual_digests[actual_digest] = actual

        shutil.copyfile(source, prototype)
        with Image.open(source) as prototype_image, Image.open(actual) as actual_image:
            canvas = Image.new("RGB", (VIEWPORT[0] * 2, VIEWPORT[1]), "#e5e7eb")
            canvas.paste(fit_on_canvas(prototype_image), (0, 0))
            canvas.paste(actual_image.convert("RGB"), (VIEWPORT[0], 0))
            canvas.save(comparison, "PNG", optimize=True)

    prototypes = list(EVIDENCE.glob("*-prototype.png"))
    actuals = list(EVIDENCE.glob("*-actual.png"))
    comparisons = list(EVIDENCE.glob("*-comparison.png"))
    if (len(prototypes), len(actuals), len(comparisons)) != (21, 21, 21):
        raise RuntimeError(
            "evidence count mismatch: "
            f"prototype={len(prototypes)} actual={len(actuals)} "
            f"comparison={len(comparisons)}"
        )
    print("STATUS: PASS")
    print("prototype=21 actual=21 comparison=21")


if __name__ == "__main__":
    main()
