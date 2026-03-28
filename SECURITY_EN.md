# Security Policy

## Reporting Security Vulnerabilities

If you discover security issues (e.g., remote code execution, privilege escalation, information disclosure, DoS, etc.), please do NOT publicly open an Issue.

Report privately via email: `1093993119@qq.com`

Email should include:

- Vulnerability description and impact scope
- Reproduction steps or PoC (if convenient, provide minimal reproduction)
- Affected version/commit information
- Possible fix suggestions (optional)

## Handling Process (Best Effort)

- We will confirm receipt as soon as possible and proceed with reproduction and evaluation.
- Upon completion of the fix, security fixes will be noted in the release notes (with upgrade suggestions for affected users when necessary).

---

## Hardening Notes (Operational Security)

Default threat model unless you harden the deployment: attackers may reach public services and any principal with bus/etcd-equivalent access. The framework does **not** replace network ACLs, mTLS, or secret management.

### Cross-process bus (`zbus` / NATS, etc.)

- Payloads are **opaque bytes**; there is **no** built-in MAC/signature on the `Message` envelope. Any writer to a topic can inject bytes that become unmarshaled messages for peers.
- **Recommend**: enable authn/authz on the broker, topic isolation, TLS; keep the bus off the public Internet unless strictly necessary.

### Service discovery (etcd, etc.)

- Tampering with registrations can misroute traffic. **Recommend**: etcd auth, TLS, least-privilege client credentials.

### JavaScript engine (`zjs`) and `require`

- **Default**: if `EngineConfig.ScriptDir` is empty, **`require()` fails** (no directory sandbox → unsafe file read surface is closed by default). Set `ScriptDir` in production to a dedicated root directory.
- **Legacy**: set `EngineConfig.AllowRequireWithoutScriptDir = true` only if you accept legacy behavior (`filepath.Abs` resolution; relative paths depend on the **process working directory**, no directory sandbox).

### RPC circuit breaker (`CallActor`)

- Breaker state is tracked per **sender Actor instance** and target `actorId`, **not** globally per downstream service. Many senders calling the same target do **not** share breaker counters; add service-wide limits elsewhere if needed.

### Examples and TLS

- Example clients may offer flags that skip certificate verification (e.g. `-gmInsecure`) for demos only. Production must verify server certificates and hostnames.

### Repository hygiene (contributors)

- Do not commit: `logs/`, `*.log`, `.env`, IDE junk, configs with real secrets. Any demo certificates in-tree must be documented as **non-production** only.
