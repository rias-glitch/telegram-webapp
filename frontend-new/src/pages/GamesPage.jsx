import { useState } from 'react'
import { Card, CardTitle } from '../components/ui'
import { CoinFlipGame } from '../components/games/CoinFlipGame'
import { RPSGame } from '../components/games/RPSGame'
import { MinesGame } from '../components/games/MinesGame'
import { GameModeSelector } from '../components/games/GameModeSelector'
import { PvPRPSGame } from '../components/games/PvPRPSGame'
import { PvPMinesGame } from '../components/games/PvPMinesGame'

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
]

export function GamesPage({ user, setUser, addGems }) {
  const [selectedGame, setSelectedGame] = useState(null)
  const [activeGame, setActiveGame] = useState(null)
  const [gameMode, setGameMode] = useState(null) // 'pve' | 'pvp'

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
    </div>
  )
}
