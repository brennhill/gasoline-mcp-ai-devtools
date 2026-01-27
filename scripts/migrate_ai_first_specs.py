from __future__ import annotations

import datetime
from pathlib import Path
import shutil


def title_from_slug(slug: str) -> str:
    return " ".join(w.capitalize() for w in slug.split("-") if w)


def ensure_scaffold(feature_dir: Path, slug: str) -> None:
    feature_dir.mkdir(parents=True, exist_ok=True)
    title = title_from_slug(slug)

    product = feature_dir / "PRODUCT_SPEC.md"
    if not product.exists():
        product.write_text(
            "\n".join(
                [
                    f"# Product Spec: {title}",
                    "",
                    f"User-facing requirements, rationale, and deprecations for the {title} feature.",
                    "",
                    "- See also: [Tech Spec](TECH_SPEC.md)",
                    f"- See also: [{title} Review]({slug}-review.md)",
                    "- See also: [Core Product Spec](../../../core/PRODUCT_SPEC.md)",
                    "",
                ]
            ),
            encoding="utf-8",
        )

    adrs = feature_dir / "ADRS.md"
    if not adrs.exists():
        adrs.write_text(
            "\n".join(
                [
                    f"# ADRs: {title}",
                    "",
                    f"Architectural decisions for the {title} feature.",
                    "",
                    "- See also: [Product Spec](PRODUCT_SPEC.md)",
                    "- See also: [Tech Spec](TECH_SPEC.md)",
                    f"- See also: [{title} Review]({slug}-review.md)",
                    "",
                ]
            ),
            encoding="utf-8",
        )


def migrate_all(repo_root: Path) -> list[tuple[str, str, Path, Path]]:
    docs = repo_root / "docs"
    ai_first = docs / "ai-first"
    archive = docs / "archive" / "ai-first"
    features_root = docs / "features" / "feature"

    today = datetime.date.today().isoformat()

    archive.mkdir(parents=True, exist_ok=True)

    migrated: list[tuple[str, str, Path, Path]] = []
    src_files = sorted(
        p
        for p in ai_first.iterdir()
        if p.is_file() and p.name.startswith("tech-spec-") and p.suffix == ".md"
    )

    for src in src_files:
        name = src.name
        slug = name[len("tech-spec-") : -len(".md")]

        feature_dir = features_root / slug
        ensure_scaffold(feature_dir, slug)

        dest = feature_dir / "TECH_SPEC.md"
        source_text = src.read_text(encoding="utf-8")

        title = title_from_slug(slug)
        banner = "\n".join(
            [
                "> **[MIGRATION NOTICE]**",
                f"> Canonical location for this tech spec. Migrated from `/docs/ai-first/{name}` on {today}.",
                f"> See also: [Product Spec](PRODUCT_SPEC.md) and [{title} Review]({slug}-review.md).",
                "",
                "",
            ]
        )

        dest.write_text(banner + source_text.lstrip("\n"), encoding="utf-8")

        archived = archive / name
        if archived.exists():
            archived.unlink()
        shutil.move(str(src), str(archived))

        migrated.append((name, slug, dest, archived))

    return migrated


if __name__ == "__main__":
    repo = Path(__file__).resolve().parents[1]
    migrated = migrate_all(repo)
    print(f"Migrated {len(migrated)} tech specs")
    for name, slug, dest, archived in migrated:
        print(f"- {name} -> {dest.relative_to(repo)} (archived {archived.relative_to(repo)})")
