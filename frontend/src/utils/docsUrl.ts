import { sanitizeUrl } from '@/utils/url'

export const INTERNAL_DOCS_URL = '/docs?page=quick-start'

/** Retired standalone documentation links now open the current site's docs. */
export function isLegacySiteDocsUrl(value: string | null | undefined): boolean {
  try {
    const url = new URL((value || '').trim())
    return (url.protocol === 'https:' || url.protocol === 'http:')
      && url.hostname === 'smallice.xyz'
      && !url.port && !url.username && !url.password
      && (url.pathname === '/docs' || url.pathname.startsWith('/docs/'))
  } catch {
    return false
  }
}

/** True when the resolved docs target points to an external http(s) site. */
export function isExternalDocsUrl(value: string | null | undefined): boolean {
  return /^https?:\/\//i.test((value || '').trim())
}

/** Keep an operator's valid external docs URL; otherwise open this site's docs. */
export function resolveDocsUrl(value: string | null | undefined): string {
  if (isLegacySiteDocsUrl(value)) return INTERNAL_DOCS_URL
  return sanitizeUrl(value || '', { allowRelative: true }) || INTERNAL_DOCS_URL
}
