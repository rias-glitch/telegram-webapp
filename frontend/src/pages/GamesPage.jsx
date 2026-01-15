import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, CardTitle, Button } from '../components/ui'
import { api } from '../api/client'
import { CoinFlipGame } from '../components/games/CoinFlipGame'
import { CoinFlipProGame } from '../components/games/CoinFlipProGame'
import { WheelGame } from '../components/games/WheelGame'
import { DiceGame } from '../components/games/DiceGame'
import { MinesProGame } from '../components/games/MinesProGame'

const games = [
  {
    id: 'coinflip-pro',
    icon: '🪙',
    title: 'Coin Flip Pro',
    description: '10 rounds, up to x100',
    multiplier: 'x100',
    navigateTo: null,
    isHot: true,
  },
  {
    id: 'rps',
    icon: '✊',
    title: 'Rock Paper Scissors',
    description: 'PvE & PvP modes',
    multiplier: 'x2',
    navigateTo: '/rps',
    hasPvP: true,
  },
  {
    id: 'mines',
    icon: '💣',
    title: 'Mines',
    description: 'PvE & PvP modes',
    multiplier: 'x2',
    navigateTo: '/mines',
    hasPvP: true,
  },
  {
    id: 'wheel',
    icon: '🎡',
    title: 'Wheel of Fortune',
    description: 'Spin to win up to 10x',
    multiplier: 'x10',
    navigateTo: null,
  },
  {
    id: 'dice',
    icon: '🎲',
    title: 'Dice',
    description: 'Predict the roll',
    multiplier: 'x100',
    navigateTo: null,
  },
  {
    id: 'mines-pro',
    icon: '💎',
    title: 'Mines Pro',
    description: 'Multi-round mines',
    multiplier: 'x24',
    navigateTo: null,
  },
]

export function GamesPage({ user, setUser, addGems }) {
  const navigate = useNavigate()
  const [activeGame, setActiveGame] = useState(null)
  const [claimingBonus, setClaimingBonus] = useState(false)

  const handleClaimBonus = useCallback(async () => {
    if (claimingBonus) return
    setClaimingBonus(true)
    try {
      await api.post('/profile/bonus')
      // Refresh user data
      const profile = await api.get('/profile')
      if (setUser) {
        setUser(prev => ({ ...prev, gems: profile.gems }))
      }
    } catch (err) {
      console.error('Failed to claim bonus:', err)
    } finally {
      setClaimingBonus(false)
    }
  }, [claimingBonus, setUser])

  const handleGameClick = (game) => {
    if (game.navigateTo) {
      navigate(game.navigateTo)
    } else {
      setActiveGame(game.id)
    }
  }

  const handleClose = () => {
    setActiveGame(null)
  }

  const handleGameResult = (newGems) => {
    if (setUser && newGems !== undefined) {
      setUser(prev => ({ ...prev, gems: newGems }))
    }
  }

  return (
    <div className="space-y-4 animate-fadeIn">
      <h1 className="text-2xl font-bold">Games</h1>

      {user && user.gems < 100 && (
        <Card className="bg-gradient-to-r from-yellow-500/20 to-orange-500/20 border-yellow-500/30">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-semibold text-yellow-400">Low balance!</p>
              <p className="text-sm text-white/60">Get 10,000 free gems</p>
            </div>
            <Button
              onClick={handleClaimBonus}
              disabled={claimingBonus}
              className="bg-yellow-500 hover:bg-yellow-600 text-black font-bold"
            >
              {claimingBonus ? 'Claiming...' : 'Claim Bonus'}
            </Button>
          </div>
        </Card>
      )}

      <div className="grid gap-3">
        {games.map((game) => (
          <Card
            key={game.id}
            onClick={() => handleGameClick(game)}
            className="flex items-center gap-4"
          >
            <div className="text-4xl">{game.icon}</div>
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <CardTitle>{game.title}</CardTitle>
                {game.isHot && (
                  <span className="text-xs bg-gradient-to-r from-orange-500 to-red-500 text-white px-2 py-0.5 rounded-full font-bold animate-pulse">
                    HOT
                  </span>
                )}
                {game.hasPvP && (
                  <span className="text-xs bg-primary/20 text-primary px-2 py-0.5 rounded-full">
                    PvP
                  </span>
                )}
              </div>
              <p className="text-white/60 text-sm">{game.description}</p>
            </div>
            <div className="bg-primary/20 text-primary px-3 py-1 rounded-full font-bold">
              {game.multiplier}
            </div>
          </Card>
        ))}
      </div>

      {/* PvE Games */}
      {activeGame === 'coinflip-pro' && (
        <CoinFlipProGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}

      {activeGame === 'coinflip' && (
        <CoinFlipGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}

      {activeGame === 'wheel' && (
        <WheelGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}

      {activeGame === 'dice' && (
        <DiceGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}

      {activeGame === 'mines-pro' && (
        <MinesProGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}
    </div>
  )
}
