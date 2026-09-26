# Private app auto-update

Status: design, 2026-09-26. Decisions confirmed by Casey 2026-09-26:

1. Device token lives in the login Keychain (not UserDefaults), MVP included.
2. Invites are multi-use by default: `max_uses=3`, 7-day expiry, both
   overridable per invite.
3. A revoked or unauthorized device shows a menu row ("Updates disabled")
   through `OnUpdateAuthFailed`; nightswatch wires it.
4. Admin tooling is Makefile targets over `wrangler d1 execute`.
5. `updates.menuet.app` is the shared mechanism for every private menuet app.

## Goal

Let a private menuet app (first: nightswatch) update itself on Macs Casey has
enrolled, and only those. Someone who learns the download URL must not be able
to install or update. Enrollment must be revocable per device.

nightswatch constraint: its Yahoo Fantasy client runs under a personal-use
agreement. "Users" are Casey's Macs and maybe family. Nothing here may make it
easy to hand nightswatch to strangers.

## Non-goals

- DRM. A person with an installed copy has the binary. We control updates, not
  execution.
- Public apps. They keep the GitHub Releases path (`AutoUpdate.Repo`).
- Staged rollout, kill switch, min-version (TODOS.md Tier 3) stay deferred.

## Existing pieces this builds on

- `menuet.go:38-73` — `AutoUpdate{Version, Repo, AllowPrerelease, FeedURL, VerifyTeamID}`.
  `RunApplication` (menuet.go:127-138) refuses `FeedURL` without `VerifyTeamID`.
- `update.go:50-55` — appcast shape `{version, url, sha256}`.
- `update.go:120-160` `checkFeed` — GET feed, fail closed, version-newer gate.
- `update.go:176-233` `installUpdate`/`prepareUpdate` — download, SHA-256,
  unzip, bundle-version binding (feed only), team pin.
- `update.go:332-364` `downloadArchive` — plain GET, 512 MiB cap.
- `update.go:545-556` `verifyCodesignTeam` — designated requirement pins Apple
  anchor + Developer ID leaf + team OU.
- `alert.go:33-51` — `Alert.Inputs` gives a text field. Enough for "paste code".
- `menuet.mk` — `sign` (143), `notarize` (155), `zip` (81), `release` (88).
- No URL-scheme support exists in menuet (no `CFBundleURLTypes`, no Apple
  Event handler).

## Q1. Where does the token live?

| Option | Security | Revocation | New-Mac UX | Cloud cost | Verdict |
|---|---|---|---|---|---|
| (a) per-download sign + notarize, token inside bundle | Same as (c): token is extractable from the bundle | Per download | Wait minutes per install | Developer ID key in cloud; Workers cannot run codesign/notarytool; needs a Linux runner with rcodesign + ASC key; notary queue | Reject |
| (b) one notarized binary, token out-of-band (Keychain) | Token extractable by the local user | Per token | Download + paste code | None extra | Adopt |
| (c) invite exchanged once for per-device credential | Same as (b) | Per device; invite can be one-time | Same as (b) | One endpoint + one table | Adopt with (b) |
| token beside the bundle (zip sidecar / xattr) | Same | Same | Lost on drag-to-Applications; xattrs depend on browser and Archive Utility | None | Reject |

Recommendation: (b)+(c). One Developer-ID-signed, notarized zip per version.
The install page gives an **invite code**. First launch shows an Alert with an
input; the app POSTs the invite and gets a **device token**, stored in the login
Keychain. The updater sends the device token as `Authorization: Bearer` on the
appcast and the zip.

Why not (a): a token inside the bundle is no harder to extract than one in the
Keychain, and it forces per-install signing and notarization. Cloudflare cannot
run Apple's tools; moving the Developer ID key to a Linux runner is a large
new secret surface for no security gain.

Why paste-code, not `nightswatch://enroll?t=…`: menuet has no URL-scheme
support. Adding it is ObjC work (Info.plist `CFBundleURLTypes` + an
`NSAppleEventManager` handler). Deferred polish; the paste flow is one alert.

Keychain vs UserDefaults: use `/usr/bin/security add-generic-password -U -a
<bundle-id> -s "menuet-update-token" -w <token>` and `find-generic-password
-w`. One exec each, no cgo, per-user, not readable by other users. UserDefaults
would be a plaintext plist. Open question: accept plist for MVP.

## Q2. Update server

Worker `updates.menuet.app` (Cloudflare Worker, TypeScript, in
`cloudflare-terraform/workers/updates/`). Storage: R2 bucket
`menuet-private-releases` (private, no public URL). Records: D1 database
`menuet-updates` (SQL gives list/revoke queries; KV would need scans).

### Records (D1)

```sql
CREATE TABLE apps (
  id TEXT PRIMARY KEY,            -- "nightswatch"
  bundle_id TEXT NOT NULL,        -- "com.github.caseymrm.nightswatch"
  latest_version TEXT,            -- "0.2.0"
  latest_key TEXT,                -- R2 key "nightswatch/0.2.0/nightswatch.zip"
  latest_sha256 TEXT
);
CREATE TABLE invites (
  token_hash TEXT PRIMARY KEY,    -- sha256(invite token)
  app_id TEXT NOT NULL,
  label TEXT,                     -- "casey-mbp", "dad"
  max_uses INTEGER NOT NULL DEFAULT 3,
  uses INTEGER NOT NULL DEFAULT 0,
  expires_at INTEGER NOT NULL,    -- unix seconds; default now+7d
  created_at INTEGER NOT NULL
);
CREATE TABLE devices (
  token_hash TEXT PRIMARY KEY,    -- sha256(device token)
  app_id TEXT NOT NULL,
  invite_hash TEXT NOT NULL,
  name TEXT NOT NULL,             -- scutil ComputerName, supplied by app
  created_at INTEGER NOT NULL,
  last_seen INTEGER,
  last_version TEXT,
  revoked_at INTEGER
);
```

Tokens are 32 random bytes, base64url. Only hashes are stored. Format:
`mi_<app>_<base64url>` for invites, `md_<app>_<base64url>` for devices, so a
leaked string is self-describing.

### Endpoints

`POST /v1/enroll`
Body `{"invite":"mi_…","name":"Casey's MacBook","version":"0.1.0"}`
200 `{"device_token":"md_…"}`
400 malformed; 404 unknown/expired/exhausted invite (same body for all three;
do not distinguish). Increments `uses`, inserts a device row.

`GET /v1/apps/{app}/appcast`  `Authorization: Bearer md_…`
200 `{"version":"0.2.0","url":"https://updates.menuet.app/v1/apps/nightswatch/download/0.2.0","sha256":"…"}`
— exactly the shape `checkFeed` parses today (update.go:51-55). Updates
`last_seen`, `last_version`.
401 no/invalid bearer; 403 revoked. 204 if `latest_version` is null.

`GET /v1/apps/{app}/download/{version}`  `Authorization: Bearer md_…`
Streams the R2 object. Same 401/403 rules. Content-Length set so the 512 MiB
cap logic is unchanged.

`PUT /v1/apps/{app}/releases/{version}`  `Authorization: Bearer <ADMIN_TOKEN>`
Body: the zip. Worker stores to R2 and computes SHA-256. Then
`POST /v1/apps/{app}/releases/{version}/publish` sets `latest_*`. Two steps so
an upload that fails mid-way never becomes "latest".

Admin endpoints (all bearer ADMIN_TOKEN): `POST /v1/apps/{app}/invites`,
`GET /v1/apps/{app}/devices`, `DELETE /v1/apps/{app}/devices/{token_hash}`.
MVP can skip these and use `wrangler d1 execute` (see Q3).

### Authorization

Bearer → sha256 → `devices` lookup → `app_id` must equal the path app →
`revoked_at IS NULL`. No token type confusion: invite tokens never authorize
appcast/download.

### Rate limiting

Cloudflare rate-limit rule on `updates.menuet.app`: 30 req/min per IP on
`/v1/enroll`, 60 req/min per IP elsewhere. Invite enumeration is 256-bit
random; rate limiting is belt-and-braces.

### Secrets

- `ADMIN_TOKEN`: Worker secret (`wrangler secret put`), source of truth in
  GCSM `casey-secrets` as `menuet-updates-admin-token`. Never in a repo.
- Device and invite tokens: only hashes in D1.
- Developer ID cert and ASC key: unchanged, local + GCSM.

### Library changes (menuet)

1. `AutoUpdate.FeedToken string` (menuet.go). When set, `checkFeed` and
   `downloadArchive` add `Authorization: Bearer <FeedToken>`. Both functions
   gain a `bearer string` parameter; empty means no header. ~15 lines.
   As built: the download carries the token only when its URL has the same
   scheme and host as `FeedURL`, so a feed cannot send the token to a host it
   names. A 204 from the feed means "no release yet", not an error.
   Requests that carry a token (feed, download, enroll) follow a redirect
   only to the exact same scheme, host, and port. The feed request sends
   `X-App-Version: <running version>` so the server can record
   `last_version`.
2. `AutoUpdate.OnUpdateAuthFailed func(status int)` — called on 401/403 from
   the feed. Default: log. nightswatch shows an "Updates disabled" menu row. ~10 lines.
3. **Bundle-ID pin.** `verifyCodesignTeam` pins team only. A compromised
   feed could serve a different Casey-signed app with a higher version.
   Add `and identifier %q` (the running bundle's `CFBundleIdentifier`, read
   with `plutil` like update.go:529-536) to the requirement. ~15 lines.
   Do this even if nothing else ships.
4. Package `github.com/caseymrm/menuet/v2/privateupdate` (pure Go):
   `Token(bundleID) (string, error)` reads Keychain; `Enroll(ctx, baseURL,
   bundleID, invite, version) error` POSTs and stores; `PromptAndEnroll`
   shows the Alert with one input and calls `Enroll`. ~120 lines.
   App wiring:
   ```go
   tok, _ := privateupdate.Token(bundleID) // "" if not enrolled
   app.AutoUpdate.FeedURL = "https://updates.menuet.app/v1/apps/nightswatch/appcast"
   app.AutoUpdate.FeedToken = tok
   app.AutoUpdate.VerifyTeamID = "AZGE7WP274"
   ```
   If `tok == ""`, the app adds an "Enroll for updates…" menu item that calls
   `PromptAndEnroll`. `RunApplication` still gates on `Version` + `FeedURL`.

Manifest signing (ed25519): keep deferred. With bearer + team pin + bundle-ID
pin + version binding, a forged manifest can only deliver a newer build signed
by Casey's Developer ID. Trigger unchanged from TODOS.md.

## Q3. Token lifecycle

- **Mint invite** (MVP): `make invite APP=nightswatch LABEL=dad` in the
  cloudflare-terraform workers dir → generates 32 random bytes locally,
  inserts hash via `wrangler d1 execute`, prints `mi_nightswatch_…` once.
  Default `max_uses=3`, `expires_at=now+7d`; `USES=` and `DAYS=` override.
- **Install page**: https://menuet.app/nightswatch/ stays link-free. The
  install "link" Casey sends is two things: a direct R2 download via the
  Worker (`GET /v1/apps/nightswatch/download/latest?invite=mi_…`, one-shot,
  consumes nothing but requires a valid unexpired invite) and the invite code
  to paste. Same invite for both. Keeps the public page and the zip separate.
- **Per-device token**: issued at enroll; never rotated in MVP.
- **List / revoke**: `make devices APP=nightswatch` and
  `make revoke APP=nightswatch DEVICE=<hash-prefix>` via `wrangler d1 execute`.
- **Revoked app**: keeps running. Feed returns 403; updater logs and, if set,
  calls `OnUpdateAuthFailed`. No self-destruct: nightswatch data is the user's
  own Yahoo data.
- **Lost token** (Keychain wiped, new Mac): mint a new invite, re-enroll from
  the menu item. Revoke the old device row.

## Q4. Release flow

Keep local. New `menuet.mk` target:

```
publish: notarize
	curl -fsS -X PUT -H "Authorization: Bearer $$MENUET_UPDATES_ADMIN_TOKEN" \
	  --data-binary @$(ZIPFILE) \
	  https://updates.menuet.app/v1/apps/$(APP)/releases/$(VERSION)
	curl -fsS -X POST -H "Authorization: Bearer $$MENUET_UPDATES_ADMIN_TOKEN" \
	  https://updates.menuet.app/v1/apps/$(APP)/releases/$(VERSION)/publish
```

`MENUET_UPDATES_ADMIN_TOKEN` is read from GCSM at invocation
(`$(shell gcloud secrets versions access latest --secret=…)`), never stored.
`VERSION` is required; the target fails if empty. Path from `make publish` to
"every enrolled Mac updates" is the existing 24 h ticker (update.go:59) — add
`AutoUpdate.CheckInterval` only if that proves too slow.

Cloud build rejected: it moves the Developer ID key off the Mac and buys
nothing for a one-person release cadence.

## Q5. Threat model

| Threat | Defends? | Notes |
|---|---|---|
| Leaked install URL (no invite) | Yes | Worker 404s without a valid invite. Page has no link. |
| Leaked invite | Partial | Valid until used once or 7 days. Revoke the device it created. |
| Leaked device token | Partial | Holder can download current and future builds until revoked. `last_seen`/version drift on the device list is the tell. Cannot forge releases. |
| Token extracted from an installed app | Same as above | By design: the local user owns the machine. Per-download signing would not change this. |
| Compromised Worker or R2 | Partial | Attacker can serve any zip but the app rejects anything not signed by team AZGE7WP274 with the same bundle ID and a newer version. Attacker can deny updates and can read hashes, device names, IPs. Cannot mint device tokens without D1 write (which a Worker compromise does give — so rotate ADMIN_TOKEN and truncate `devices` after any incident). |
| Compromised Developer ID key | No | Out of scope; same as every Mac app. |
| Yahoo-terms risk (stranger gets nightswatch) | Reduced | Only via a Casey-issued invite; each is labelled, capped (default 3 uses), and expires in 7 days. A stranger with a copied bundle runs it but never updates, and needs their own Yahoo OAuth. |

## Q6. Scope

MVP (serves nightswatch on Casey's Macs):
- menuet: `FeedToken`, bearer on feed+download, bundle-ID pin,
  `OnUpdateAuthFailed`, `privateupdate` package, tests in `feedupdate_test.go`
  style. ~250 lines + tests. Tag v2.15.0.
- Worker: enroll, appcast, download, upload/publish. ~200 lines TS + wrangler.toml
  + D1 schema. Terraform: route + R2 bucket + D1 binding (extend `r2.tf`,
  `workers/`).
- Admin: Makefile targets wrapping `wrangler d1 execute` (invite, devices,
  revoke). No admin web page.
- nightswatch: wire `AutoUpdate`, add "Enroll for updates…" menu item, set
  `IDENTITY` default to Developer ID, add `make publish`.

Deferred:
- Custom URL scheme enrollment (ObjC in menuet).
- Admin HTTP endpoints and an admin page.
- ed25519 manifest signing (trigger unchanged).
- Token rotation, device token expiry.
- Tier-3 controls; `latest_*` columns are where a kill switch would go.

## Error-first checklist

- Feed 401/403: log, hook, no update, no retry storm (ticker is 24 h).
- Enroll 404: alert "Invite not valid"; token unchanged.
- Keychain read fails: treat as not enrolled; show enroll item.
- `make publish` with empty `VERSION` or missing admin token: fail before upload.
- Upload without publish: `latest_*` unchanged; no client sees it.
