import { describe, expect, it } from 'vitest'
import { INTERNAL_DOCS_URL, isExternalDocsUrl, isLegacySiteDocsUrl, resolveDocsUrl } from '../docsUrl'

describe('documentation URL selection', () => {
  it('falls back to local docs without a configured URL', () => {
    expect(resolveDocsUrl('')).toBe(INTERNAL_DOCS_URL)
    expect(resolveDocsUrl(undefined)).toBe(INTERNAL_DOCS_URL)
  })
  it.each([
    'https://smallice.xyz/docs/',
    'https://smallice.xyz/docs/docs/guide/smallice-ai.html?v=20260720-001707',
  ])('keeps migrated documentation on the current origin: %s', (legacyUrl) => {
    expect(isLegacySiteDocsUrl(legacyUrl)).toBe(true)
    expect(resolveDocsUrl(legacyUrl)).toBe(INTERNAL_DOCS_URL)
    for (const origin of ['https://green.2jarvis.com', 'https://ai2jarvis.com']) {
      expect(new URL(resolveDocsUrl(legacyUrl), origin).origin).toBe(origin)
    }
  })
  it.each(['https://docs.example.test/docs/', 'https://smallice.xyz/other', 'https://smallice.xyz/docs-other', 'https://smallice.xyz.evil.test/docs/'])('does not replace other configured destinations: %s', (url) => {
    expect(isLegacySiteDocsUrl(url)).toBe(false)
    expect(resolveDocsUrl(url)).toBe(url)
  })
  it('retains valid configured external and local URLs', () => {
    expect(resolveDocsUrl('https://docs.example.test/guide')).toBe('https://docs.example.test/guide')
    expect(resolveDocsUrl('/guide?page=start')).toBe('/guide?page=start')
  })
  it('distinguishes external http(s) docs URLs from internal SPA docs paths', () => {
    expect(isExternalDocsUrl('https://docs.example.test/guide')).toBe(true)
    expect(isExternalDocsUrl('http://docs.example.test/guide')).toBe(true)
    expect(isExternalDocsUrl('/docs?page=quick-start')).toBe(false)
    expect(isExternalDocsUrl('')).toBe(false)
  })
  it('cannot turn a docs link into executable or protocol-relative content', () => {
    for (const value of ['javascript:alert(1)', 'data:text/html,<script>', '//evil.example.test/docs']) {
      expect(resolveDocsUrl(value)).toBe(INTERNAL_DOCS_URL)
    }
  })
})
