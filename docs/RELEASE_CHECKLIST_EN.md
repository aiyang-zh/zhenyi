# Release Checklist

This checklist is used for unified final inspection before each `zhenyi` release, covering documentation, quality, version, and release actions.

## 1. Documentation Final Check (Required)

- [ ] All repository links accessible (`README.md` + `docs/*.md` + each module `README*.md`)
- [ ] Terminology consistency check passed: unified use of "real-time application solution"
- [ ] Module positioning consistent with main README (module responsibilities, layer relationships, capability boundaries)
- [ ] New/modified capabilities have entries in `docs/DOCS_INDEX.md`
- [ ] License description consistent (AGPL-3.0 + Commercial Dual License, no conflicts with each repository boundary)

## 2. Quality Check (Required)

- [ ] `go test ./... -count=1` passes
- [ ] `make test` passes (if environment provides)
- [ ] CI status all green (PR checks / default branch checks)
- [ ] No blocking known issues (P0/P1)

## 3. Version and Compatibility (Required)

- [ ] Version number confirmed (example: `v0.1.0`)
- [ ] Release notes updated (`docs/RELEASE_NOTES_v*.md`)
- [ ] Known limitations, usage boundaries, migration suggestions clearly documented
- [ ] External API changes have compatibility impact declared

## 4. GitHub Release Preparation (Required)

- [ ] Tag name follows semantic versioning (`vMAJOR.MINOR.PATCH`)
- [ ] Release title matches tag (e.g., `v0.1.0`)
- [ ] Release body includes: positioning, core capabilities, known limitations, usage boundaries, migration suggestions, license guidance
- [ ] Release includes verification info (optional: key commits, test commands)

## 5. Suggested Commands (Directly Reusable)

```bash
# 1) Documentation link accessibility (local quick check)
python3 - <<'PY'
import re
from pathlib import Path
root = Path(".")
files = [*root.glob("README*.md"), *root.glob("docs/*.md"), *root.glob("*/README*.md")]
pat = re.compile(r'\[[^\]]+\]\(([^)]+)\)')
bad = []
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
print("BROKEN:", len(bad))
for i in bad:
    print(i[0], "->", i[1])
PY

# 2) Terminology consistency spot check
rg "real-time application solution|Solution Layer" README.md docs *.md */README*.md

# 3) Test
go test ./... -count=1
```

## 6. Suggested Baseline for Initial Release

- Recommended first version: `v0.1.0`
- Initial release documentation: `docs/RELEASE_NOTES_v0.md`
- Final check conclusion template: Pass / Conditional Pass / Blocking (with blocking items attached)
