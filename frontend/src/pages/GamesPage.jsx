import { useState, useCallback } from 'react'
import { Card, CardTitle, Button } from '../components/ui'
import { api } from '../api/client'
import { CoinFlipGame } from '../components/games/CoinFlipGame'
import { RPSGame } from '../components/games/RPSGame'
import { MinesGame } from '../components/games/MinesGame'
import { GameModeSelector } from '../components/games/GameModeSelector'
import { PvPRPSGame } from '../components/games/PvPRPSGame'
import { PvPMinesGame } from '../components/games/PvPMinesGame'
import { WheelGame } from '../components/games/WheelGame'
import { DiceGame } from '../components/games/DiceGame'
import { MinesProGame } from '../components/games/MinesProGame'

const games = [
  {
    id: 'coinflip',
    icon: '🪙',
    title: 'Coin Flip',
    description: '50/50 chance to double',
    multiplier: 'x2',
    hasPvP: false,
  },
  {
    id: 'rps',
    icon: '✊',
    title: 'Rock Paper Scissors',
    description: 'Beat the opponent',
    multiplier: 'x2',
    hasPvP: true,
  },
  {
    id: 'mines',
    icon: '💣',
    title: 'Mines',
    description: 'Avoid the bombs',
    multiplier: 'x2',
    hasPvP: true,
  },
  {
    id: 'wheel',
    icon: '🎡',
    title: 'Wheel of Fortune',
    description: 'Spin to win up to 10x',
    multiplier: 'x10',
    hasPvP: false,
  },
  {
    id: 'dice',
    icon: '🎲',
    title: 'Dice',
    description: 'Predict the roll',
    multiplier: 'x100',
    hasPvP: false,
  },
  {
    id: 'mines-pro',
    icon: '💎',
    title: 'Mines Pro',
    description: 'Multi-round mines',
    multiplier: 'x24',
    hasPvP: false,
  },
]

export function GamesPage({ user, setUser, addGems }) {
  const [selectedGame, setSelectedGame] = useState(null)
  const [activeGame, setActiveGame] = useState(null)
  const [gameMode, setGameMode] = useState(null) // 'pve' | 'pvp'
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
    if (game.hasPvP) {
      setSelectedGame(game)
    } else {
      setActiveGame(game.id)
    }
  }

  const handleModeSelect = (mode) => {
    setGameMode(mode)
    setActiveGame(selectedGame.id)
    setSelectedGame(null)
  }

  const handleClose = () => {
    setActiveGame(null)
    setSelectedGame(null)
    setGameMode(null)
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

      {/* Mode selector for PvP games */}
      {selectedGame && (
        <GameModeSelector
          isOpen={true}
          onClose={() => setSelectedGame(null)}
          onSelect={handleModeSelect}
          gameTitle={selectedGame.title}
        />
      )}

      {/* PvE Games */}
      {activeGame === 'coinflip' && (
        <CoinFlipGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}

      {activeGame === 'rps' && gameMode === 'pve' && (
        <RPSGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}

      {activeGame === 'mines' && gameMode === 'pve' && (
        <MinesGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}

      {/* PvP Games */}
      {activeGame === 'rps' && gameMode === 'pvp' && (
        <PvPRPSGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}

      {activeGame === 'mines' && gameMode === 'pvp' && (
        <PvPMinesGame
          user={user}
          onClose={handleClose}
          onResult={handleGameResult}
        />
      )}

      {/* New PvE Games */}
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
