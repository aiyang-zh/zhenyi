# Documentation Index

This page provides quick navigation to `zhenyi`'s core documentation, organized by "Getting Started -> Architecture -> Capabilities -> Observability -> Operations".

The root [README.md](../README_EN.md) stays short; **this page is the full index** (sections and links below). For **documentation notes, CI, and licensing**, see the root README footer and [LICENSE](../LICENSE). If documentation disagrees with reproducible behavior, prefer the source and tests here; issues and PRs welcome.

## 1. Getting Started & Overview

- [Book: Go Actor model & real-time apps (`docs/books/go-actor-realtime`)](books/go-actor-realtime/README.md) (read alongside source; license in book README)
- [Commercial License](../COMMERCIAL_LICENSE.md) (Complete guidance for network services to external parties)
- [Support](../SUPPORT_EN.md)
- [Security Policy & Hardening](../SECURITY_EN.md)
- [Beginner's Guide](BEGINNER_GUIDE.md)
- [Architecture](ARCHITECTURE.md)
- [Module API Navigation](MODULE_API.md)
- [Examples Overview](EXAMPLES.md)
- [Xinchuang Adaptation Analysis & Initial Validation](XINCHUANG.md) (Domestic environment: Linux/arm64, no CGO, cross-compilation suggestions)

## 2. Core Capability Documentation

- [Global Variables & Hooks / Startup Checks](GLOBALS_AND_HOOKS.md)
- [Third-Party Licenses](../THIRD_PARTY_LICENSES_EN.md)
- [Monitoring & Observability Overview](MONITORING_OVERVIEW.md) (includes **section 4 optional Pyroscope** continuous profiling)
- [Monitoring Metrics Details](MONITORING_METRICS.md)
- [Codec Adapters (IMessage)](CODEC_ADAPTERS.md)
- [TLS/GM-TLS (National Secret)](../zgate/README.md) (Gateway encryption: `SetTLSConfig` / `SetGMTLS`)
- [Release Checklist](RELEASE_CHECKLIST.md)
- [Initial Release Notes (v0)](RELEASE_NOTES_v0.md)

## 3. Module README Navigation

- Startup orchestration: `zstartup/README.md`
- Gateway entry: `zgate/README.md`
- Actor runtime: `zactor/README.md`
- Messages & models: `zmsg/README.md`, `zmodel/README.md`
- Routing & discovery: `zroute/README.md`, `zdiscovery/README.md`
- Observability: `zmetrics/README.md`, `zmonitor/README.md`, `ztrace/README.md`, `zpyroscope/README.md`
- Bus & integration: `zbus/README.md`, `znats/README.md`

## 4. Maintenance Suggestions

- After code interface changes, sync update corresponding module README and this index
- When adding new modules, add entries to both `README.md` and this index
- Use code as the source of truth; avoid keeping outdated example signatures
