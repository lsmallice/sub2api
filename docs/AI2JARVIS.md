# AI2Jarvis Distribution

This branch adapts Kedaya v2.4.3 for AI2Jarvis. Upstream copyright notices and the
LGPL license are retained. The original source snapshot is preserved on
`kedaya-v2.4.3`; development takes place on `codex/ai2jarvis-main`.

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
