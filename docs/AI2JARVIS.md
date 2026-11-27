# AI2Jarvis Distribution

This branch adapts Kedaya v2.4.4 for AI2Jarvis. Upstream copyright notices and the
LGPL license are retained. The original source snapshot is preserved on
`kedaya-v2.4.3`; development takes place on `codex/ai2jarvis-main`.

## 2.4.4 Integration

The upstream release tag is `v2.4.4`, pointing to commit
`a214cf1f82d7ca221d726d5cf44ad825ea818156`. The distribution version is
`2.4.4-ai2jarvis.1`; source builds must not inherit the older upstream VERSION
file value.

The merge includes upstream disconnect billing fixes, response ID search,
personal management API keys, backup encryption and rate limits, and channel
monitor latency corrections. Existing branding, the opt-in Infinite Canvas menu,
same-origin tutorial navigation, omitted platform-quota preservation, and the
optional legacy payment-envelope compatibility remain in place.

Before deploying, back up and validate migrations 346 through 349. They add group
and usage-log fields, concurrent usage-log indexes, and the personal API-key table.
Keep production migration mode at `validate`; applying migrations and directing
traffic to this build are separate deployment steps, not part of the source merge.

Validation includes 343 frontend assertions, frontend type checking and production
build, backend build and vet, and focused backend unit tests for the new features
and retained compatibility. One upstream socket-timing test remains unresolved:
`TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient_ClientDisconnectClosesUpstreamAndEstimates`.
It also fails intermittently on an untouched `v2.4.4` checkout on macOS, sometimes
consuming the terminal event before observing the closed client. It was excluded
from the otherwise passing focused backend run; no WebSocket runtime changes or
weakened test assertions were introduced in this merge.

## Site Configuration

The administrator's public settings remain authoritative for the site name,
subtitle, logo, API base URL, documentation URL, and support contact. The initial
browser title and fallback name are AI2Jarvis. Homepage and documentation examples
use the configured API base URL, including when the console and API use different
domains. Logo MIME types are not assumed to be SVG.

For an isolated acceptance environment, configure its own API base URL and a
visible environment suffix in the site name. Do not copy payment availability or
enable background work against a production snapshot.

## Deployment Status

The frontend adaptation does not by itself certify migration from official
Sub2API. The first transition must address credential-storage compatibility,
administrator API access for integrations, payment calculations, and rollback.
After migration, blue/green candidates must validate the shared schema, preserve
concurrency slots, and coordinate background workers before receiving traffic.

Do not replace the production main branch or use a mutable image tag as part of
this branding change.
