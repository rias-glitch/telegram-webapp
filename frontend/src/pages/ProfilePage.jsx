import { useEffect, useState } from 'react'
import { Card, CardTitle, Button } from '../components/ui'
import { claimQuestReward } from '../api/quests'

const GAME_ICONS = {
  coinflip: '🪙',
  rps: '✊',
  mines: '💣',
  case: '🎁',
}

const RESULT_COLORS = {
  win: 'text-success',
  lose: 'text-danger',
  draw: 'text-yellow-400',
}

export function ProfilePage({ user, games, stats, quests, fetchProfile }) {
  const [activeTab, setActiveTab] = useState('stats')
  const [claiming, setClaiming] = useState(null)

  useEffect(() => {
    fetchProfile()
  }, [fetchProfile])

  const handleClaim = async (userQuestId) => {
    try {
      setClaiming(userQuestId)
      await claimQuestReward(userQuestId)
      fetchProfile()
    } catch (err) {
      console.error('Failed to claim reward:', err)
    } finally {
      setClaiming(null)
    }
  }

  const formatDate = (dateStr) => {
    const date = new Date(dateStr)
    return date.toLocaleDateString('ru-RU', {
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  return (
    <div className="space-y-4 animate-fadeIn">
      {/* Profile header */}
      <Card className="text-center">
        <div className="text-5xl mb-3">👤</div>
        <h2 className="text-xl font-bold">
          {user?.first_name || user?.username || 'Player'}
        </h2>
        {user?.username && (
          <p className="text-white/60">@{user.username}</p>
        )}
        <div className="flex items-center justify-center gap-2 mt-3">
          <span className="text-2xl">💎</span>
          <span className="text-2xl font-bold">{user?.gems?.toLocaleString() || 0}</span>
        </div>
      </Card>

      {/* Tabs */}
      <div className="flex gap-2">
        {['stats', 'history', 'quests'].map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`flex-1 py-2 rounded-xl font-medium transition-colors ${
              activeTab === tab
                ? 'bg-primary text-white'
                : 'bg-white/10 text-white/60 hover:bg-white/20'
            }`}
          >
            {tab === 'stats' && 'Stats'}
            {tab === 'history' && 'History'}
            {tab === 'quests' && 'Quests'}
          </button>
        ))}
      </div>

      {/* Stats tab */}
      {activeTab === 'stats' && stats && (
        <Card>
          <CardTitle className="mb-4">Monthly Statistics</CardTitle>
          <div className="grid grid-cols-2 gap-4">
            <div className="text-center p-3 bg-white/5 rounded-xl">
              <div className="text-2xl font-bold">{stats.total_games || 0}</div>
              <div className="text-white/60 text-sm">Games</div>
            </div>
            <div className="text-center p-3 bg-white/5 rounded-xl">
              <div className="text-2xl font-bold text-success">{stats.wins || 0}</div>
              <div className="text-white/60 text-sm">Wins</div>
            </div>
            <div className="text-center p-3 bg-white/5 rounded-xl">
              <div className="text-2xl font-bold text-danger">{stats.losses || 0}</div>
              <div className="text-white/60 text-sm">Losses</div>
            </div>
            <div className="text-center p-3 bg-white/5 rounded-xl">
              <div className="text-2xl font-bold">
                {stats.total_games > 0
                  ? Math.round((stats.wins / stats.total_games) * 100)
                  : 0}%
              </div>
              <div className="text-white/60 text-sm">Win Rate</div>
            </div>
          </div>
          <div className="mt-4 pt-4 border-t border-white/10">
            <div className="flex justify-between">
              <span className="text-white/60">Total Won</span>
              <span className="text-success font-bold">+{stats.total_won?.toLocaleString() || 0}</span>
            </div>
            <div className="flex justify-between mt-2">
              <span className="text-white/60">Total Lost</span>
              <span className="text-danger font-bold">-{stats.total_lost?.toLocaleString() || 0}</span>
            </div>
          </div>
        </Card>
      )}

      {/* History tab */}
      {activeTab === 'history' && (
        <div className="space-y-2">
          {(!games || games.length === 0) ? (
            <Card className="text-center py-8">
              <div className="text-4xl mb-2">🎮</div>
              <p className="text-white/60">No games yet</p>
            </Card>
          ) : (
            games.slice(0, 20).map((game) => (
              <Card key={game.id} className="flex items-center gap-3">
                <div className="text-2xl">
                  {GAME_ICONS[game.game_type] || '🎮'}
                </div>
                <div className="flex-1">
                  <div className="font-medium capitalize">{game.game_type}</div>
                  <div className="text-white/40 text-xs">{formatDate(game.created_at)}</div>
                </div>
                <div className="text-right">
                  <div className={`font-bold ${RESULT_COLORS[game.result]}`}>
                    {game.result === 'win' && '+'}
                    {game.result === 'lose' && '-'}
                    {game.win_amount !== 0 ? Math.abs(game.win_amount) : game.bet_amount}
                  </div>
                  <div className="text-white/40 text-xs capitalize">{game.result}</div>
                </div>
              </Card>
            ))
          )}
        </div>
      )}

      {/* Quests tab */}
      {activeTab === 'quests' && (
        <div className="space-y-3">
          {(!quests || quests.length === 0) ? (
            <Card className="text-center py-8">
              <div className="text-4xl mb-2">📋</div>
              <p className="text-white/60">No active quests</p>
            </Card>
          ) : (
            quests.map((q) => (
              <Card key={q.quest?.id || q.user_quest_id}>
                <div className="flex items-start justify-between mb-2">
                  <div>
                    <div className="font-semibold">{q.quest?.title}</div>
                    <div className="text-white/60 text-sm">{q.quest?.description}</div>
                  </div>
                  <div className="flex items-center gap-1 bg-primary/20 text-primary px-2 py-1 rounded-lg">
                    <span>💎</span>
                    <span className="font-bold">{q.quest?.reward_gems}</span>
                  </div>
                </div>

                {/* Progress bar */}
                <div className="mt-3">
                  <div className="flex justify-between text-sm mb-1">
                    <span className="text-white/60">Progress</span>
                    <span>{q.current_count || 0}/{q.target_count}</span>
                  </div>
                  <div className="h-2 bg-white/10 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-gradient-to-r from-primary to-secondary transition-all duration-300"
                      style={{ width: `${Math.min(q.progress || 0, 100)}%` }}
                    />
                  </div>
                </div>

                {/* Claim button */}
                {q.completed && !q.reward_claimed && q.user_quest_id && (
                  <Button
                    onClick={() => handleClaim(q.user_quest_id)}
                    disabled={claiming === q.user_quest_id}
                    size="sm"
                    className="w-full mt-3"
                  >
                    {claiming === q.user_quest_id ? 'Claiming...' : 'Claim Reward'}
                  </Button>
                )}

                {q.reward_claimed && (
                  <div className="text-center text-success text-sm mt-3">
                    Reward claimed!
                  </div>
                )}
              </Card>
            ))
          )}
        </div>
      )}
    </div>
  )
}
