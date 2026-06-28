const DEFAULT_DRAW_BASE_URL = 'https://draw.smallice.xyz'

export function buildDrawLaunchURL(token: string): string {
  const cleanToken = token.trim()
  if (!cleanToken) {
    throw new Error('登录状态已失效，请重新登录')
  }
  const baseURL = getDrawBaseURL()
  if (!baseURL) {
    throw new Error('Draw URL is not configured')
  }
  const url = new URL(baseURL)
  url.searchParams.set('theme', currentThemeName())
  url.hash = `token=${encodeURIComponent(cleanToken)}`
  return url.toString()
}

export function getDrawBaseURL(): string {
  const configuredURL = import.meta.env.VITE_SMALLICE_DRAW_URL?.trim()
  return trimTrailingSlash(configuredURL || DEFAULT_DRAW_BASE_URL)
}

function currentThemeName(): 'light' | 'dark' {
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, '')
}
