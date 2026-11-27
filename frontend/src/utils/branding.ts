import { sanitizeUrl } from '@/utils/url'

export const DEFAULT_SITE_NAME = 'AI2Jarvis'

export function updateFavicon(logoUrl: string): void {
  if (typeof document === 'undefined') return
  const sanitizedLogoUrl = sanitizeUrl(logoUrl, {
    allowRelative: true,
    allowDataUrl: true,
  })
  if (!sanitizedLogoUrl) {
    return
  }

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  // /site-logo serves the configured image format, including JPEG and PNG.
  link.removeAttribute('type')
  link.href = sanitizedLogoUrl
}
