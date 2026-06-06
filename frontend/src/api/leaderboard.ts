import { apiClient } from './client'
import type {
  LeaderboardOverview,
  LeaderboardParticipantStatus,
  UpdateLeaderboardParticipantRequest,
} from '@/types'

export async function getLeaderboardOverview(): Promise<LeaderboardOverview> {
  const { data } = await apiClient.get<LeaderboardOverview>('/leaderboard/overview')
  return data
}

export async function getLeaderboardMe(): Promise<LeaderboardParticipantStatus> {
  const { data } = await apiClient.get<LeaderboardParticipantStatus>('/leaderboard/me')
  return data
}

export async function updateLeaderboardMe(
  payload: UpdateLeaderboardParticipantRequest
): Promise<LeaderboardParticipantStatus> {
  const { data } = await apiClient.put<LeaderboardParticipantStatus>('/leaderboard/me', payload)
  return data
}

export const leaderboardAPI = {
  getOverview: getLeaderboardOverview,
  getMe: getLeaderboardMe,
  updateMe: updateLeaderboardMe,
}

export default leaderboardAPI
