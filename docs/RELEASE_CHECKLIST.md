# 发布终检清单（Release Checklist）

本清单用于 `zhenyi` 每次发布前的统一终检，覆盖文档、质量、版本与发布动作。

## 1. 文档终检（必须）

- [ ] 全仓链接可达检查通过（`README.md` + `docs/*.md` + 各模块 `README*.md`）
- [ ] 术语一致性检查通过：统一使用“实时应用解决方案”口径
- [ ] 模块定位与主 README 口径一致（模块职责、层次关系、能力边界）
- [ ] 新增/修改能力在 `docs/DOCS_INDEX.md` 中有入口
- [ ] 许可证描述一致（AGPL-3.0 + 商业双授权，不与各仓边界冲突）

## 2. 质量检查（必须）

- [ ] `go test ./... -count=1` 通过
- [ ] `make test` 通过（若环境提供）
- [ ] CI 状态全绿（PR checks / default branch checks）
- [ ] 无阻塞级已知问题（P0/P1）

## 3. 版本与兼容性（必须）

- [ ] 版本号已确定（示例：`v0.1.0`）
- [ ] 发布说明已更新（`docs/RELEASE_NOTES_v*.md`）
- [ ] 已知限制、使用边界、迁移建议已写清
- [ ] 对外 API 变更已声明兼容性影响

## 4. GitHub Release 准备（必须）

- [ ] Tag 命名符合语义化版本（`vMAJOR.MINOR.PATCH`）
- [ ] Release 标题与 Tag 一致（例如 `v0.1.0`）
- [ ] Release 正文包含：定位、核心能力、已知限制、使用边界、迁移建议、许可证口径
- [ ] Release 附带校验信息（可选：关键 commit、测试命令）

## 5. 建议命令（可直接复用）

```bash
# 1) 文档链接可达（本地快速检查）
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

# 2) 术语一致性抽查
rg "实时应用解决方案|解决方案层" README.md docs *.md */README*.md

# 3) 测试
go test ./... -count=1
```

## 6. 本次首发建议基线

- 建议首个版本号：`v0.1.0`
- 首发文档：`docs/RELEASE_NOTES_v0.md`
- 终检结论模板：通过 / 有条件通过 / 阻塞（附阻塞项）
