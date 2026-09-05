# Changelog

## 2026-09 — Console ticket issuance and terminal deprecation headers (V2 Phase -1, Release A)

### Changed

- **`GET /api/v1/asset/service/diagnosis/run` now answers `410 Gone`** with
  `Allow: POST` and `Location: /api/v1/asset/service/diagnosis/run` headers.
  The same diagnosis run is available at **`POST /api/v1/asset/service/diagnosis/run`**
  (JSON body, identical fields to the former query string; the response shape
  is unchanged). **External scripts and saved bookmarks that still call the
  GET form will receive 410 and must switch to POST.** The bundled web UI has
  been switched to POST in the same release and needs no action. The required
  permission (`assets:service:diagnosis`) is unchanged, so existing grants
  carry over.

### Added

- **`POST /api/v1/console-sessions`** mints a one-time terminal ticket
  (30-second TTL, single use) bound to exactly one resource and protocol.
  Requires `assets:host:terminal` or `assets:k8s:pod:terminal` (the handler
  re-verifies the permission matching the requested resource). Tickets are
  presented as `?ticket=` on the terminal websocket endpoints.

### Deprecated

- **The legacy query-token path of the terminal websockets**
  (`GET /api/v1/asset/terminal/ws?token=…` and
  `GET /api/v1/k8s/pod/terminal/ws?token=…`) keeps working but is scheduled
  for removal. Handshakes through it now carry `Deprecation: true` and a
  `Sunset` header. Clients should mint a ticket via
  `POST /api/v1/console-sessions` and connect with `?ticket=` instead. A
  ticket parameter, when present, is consumed atomically: invalid, expired or
  reused tickets are rejected with 401 and never fall back to the token path,
  and tickets presented against a different resource or protocol are rejected
  with 403.
