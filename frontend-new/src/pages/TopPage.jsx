import { useState, useEffect } from 'react'
import { Card } from '../components/ui'
import { getTopUsers } from '../api/games'

const MEDALS = ['🥇', '🥈', '🥉']

export function TopPage() {
  const [users, setUsers] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    loadTop()
  }, [])

  const loadTop = async () => {
    try {
      setLoading(true)
      const response = await getTopUsers()
      setUsers(response.top || [])
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <div className="text-4xl mb-2 animate-pulse-custom">🏆</div>
          <p className="text-white/60">Loading leaderboard...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <div className="text-4xl mb-2">❌</div>
        <p className="text-danger">{error}</p>
      </div>
    )
  }

  return (
    <div className="space-y-4 animate-fadeIn">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Leaderboard</h1>
        <span className="text-white/60 text-sm">Monthly wins</span>
      </div>

      {users.length === 0 ? (
        <Card className="text-center py-8">
          <div className="text-4xl mb-2">🏆</div>
          <p className="text-white/60">No players yet</p>
        </Card>
      ) : (
        <div className="space-y-2">
          {users.map((user, index) => (
            <Card
              key={user.user_id}
              className={`flex items-center gap-3 ${
                index < 3 ? 'border border-primary/30' : ''
              }`}
            >
              {/* Rank */}
              <div className="w-10 text-center">
                {index < 3 ? (
                  <span className="text-2xl">{MEDALS[index]}</span>
                ) : (
                  <span className="text-white/40 font-bold">{index + 1}</span>
                )}
              </div>

              {/* User info */}
              <div className="flex-1 min-w-0">
                <div className="font-semibold truncate">
                  {user.first_name || user.username || `User ${user.user_id}`}
                </div>
                {user.username && user.first_name && (
                  <div className="text-white/40 text-sm truncate">@{user.username}</div>
                )}
              </div>

              {/* Stats */}
              <div className="text-right">
                <div className="font-bold text-success">{user.wins} wins</div>
                <div className="text-white/40 text-xs">{user.games} games</div>
              </div>

              {/* Gems */}
              <div className="flex items-center gap-1 bg-white/10 px-2 py-1 rounded-lg">
                <span className="text-sm">💎</span>
                <span className="font-bold text-sm">{user.gems?.toLocaleString()}</span>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
