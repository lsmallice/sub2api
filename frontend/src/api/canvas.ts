import { apiClient } from './client'

export interface CanvasSSOTicket {
  ticket: string
  redirect_url: string
  expires_at: string
}

export async function createSSOTicket(): Promise<CanvasSSOTicket> {
  const { data } = await apiClient.post<CanvasSSOTicket>('/canvas/sso-ticket')
  return data
}

export const canvasAPI = {
  createSSOTicket,
}
