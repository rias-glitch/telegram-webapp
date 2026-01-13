import { useState, useCallback } from 'react'
import { getMyProfile, updateBalance } from '../api/auth'
import { getMyGames } from '../api/games'
import { getMyQuests } from '../api/quests'

export function useProfile(user, setUser) {
  const [games, setGames] = useState([])
  const [stats, setStats] = useState(null)
  const [quests, setQuests] = useState([])
  const [loading, setLoading] = useState(false)

  const fetchProfile = useCallback(async () => {
    if (!user) return
    try {
      setLoading(true)
      const [gamesData, questsData] = await Promise.all([
        getMyGames(),
        getMyQuests(),
      ])
      setGames(gamesData.games || [])
      setStats(gamesData.stats)
      setQuests(questsData.quests || [])
    } catch (err) {
      console.error('Failed to fetch profile:', err)
    } finally {
      setLoading(false)
    }
  }, [user])

  const addGems = useCallback(async (amount, reason = 'game') => {
    try {
      const response = await updateBalance(amount, reason)
      if (setUser && response.new_balance !== undefined) {
        setUser(prev => ({ ...prev, gems: response.new_balance }))
      }
      return response
    } catch (err) {
      console.error('Failed to update balance:', err)
      throw err
    }
  }, [setUser])

  return { games, stats, quests, loading, fetchProfile, addGems }
}
