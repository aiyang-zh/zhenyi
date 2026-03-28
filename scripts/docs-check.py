import re
from pathlib import Path
from typing import List, Tuple


def collect_markdown_files(root: Path) -> List[Path]:
    patterns = (
        "README*.md",
        "docs/**/*.md",
        "examples/**/*.md",
        "*/README*.md",
    )
    seen: set = set()
    out: list[Path] = []
    for pat in patterns:
        for p in root.glob(pat):
            if p.is_file() and p not in seen:
                seen.add(p)
                out.append(p)
    return sorted(out, key=lambda x: str(x))


def main() -> int:
    root = Path(".")
    files = collect_markdown_files(root)
    pat = re.compile(r"\[[^\]]+\]\(([^)]+)\)")

    bad: List[Tuple[str, str]] = []
    for f in files:
        txt = f.read_text(encoding="utf-8", errors="ignore")
        for m in pat.finditer(txt):
            url = m.group(1).strip()
            if (not url) or url.startswith("#") or "://" in url or url.startswith("mailto:"):
                continue
            p = url.split("#", 1)[0].split("?", 1)[0].strip()
            if not p:
                continue
            if not (f.parent / p).resolve().exists():
                bad.append((str(f), url))

    print("SCANNED:", len(files), "markdown files")
    print("BROKEN:", len(bad))
    for fp, url in bad:
        print(fp, "->", url)
    return 1 if bad else 0


if __name__ == "__main__":
    raise SystemExit(main())
