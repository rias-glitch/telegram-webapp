import { useState, useEffect } from 'react'
import { Modal } from '../ui/Overlay'
import { Button } from '../ui'
import { useWebSocket } from '../../hooks/useWebSocket'

const MOVES = [
  { id: 'rock', icon: '🪨', label: 'Rock' },
  { id: 'paper', icon: '📄', label: 'Paper' },
  { id: 'scissors', icon: '✂️', label: 'Scissors' },
]

export function PvPRPSGame({ user, onClose, onResult }) {
  const {
    status,
    opponent,
    roomId,
    gameState,
    result,
    connect,
    send,
    disconnect,
  } = useWebSocket('rps')

  const [selectedMove, setSelectedMove] = useState(null)
  const [waiting, setWaiting] = useState(false)

  useEffect(() => {
    // Auto connect when component mounts
    connect(0) // bet = 0 for now

    return () => {
      disconnect()
    }
  }, [])

  useEffect(() => {
    if (result) {
      // Game finished
      setWaiting(false)
    }
  }, [result])

  // Reset state when new round starts (gameState changes or status becomes 'playing')
  useEffect(() => {
    if (status === 'playing' && gameState) {
      // New round started - reset selection
      setSelectedMove(null)
      setWaiting(false)
    }
  }, [gameState, status])

  const handleMove = (move) => {
    setSelectedMove(move)
    setWaiting(true)
    send({ type: 'move', value: move })
  }

  const handlePlayAgain = () => {
    setSelectedMove(null)
    setWaiting(false)
    disconnect()
    setTimeout(() => connect(0), 100)
  }

  const getMoveIcon = (move) => MOVES.find(m => m.id === move)?.icon || '❓'

  const getResultText = () => {
    if (!result?.payload) return ''
    const you = result.payload.you
    if (you === 'win') return 'YOU WON!'
    if (you === 'lose') return 'YOU LOST'
    return 'DRAW'
  }

  const getResultColor = () => {
    if (!result?.payload) return ''
    const you = result.payload.you
    if (you === 'win') return 'text-success'
    if (you === 'lose') return 'text-danger'
    return 'text-yellow-400'
  }

  return (
    <Modal isOpen={true} onClose={onClose} title="PvP Rock Paper Scissors">
      <div className="space-y-6">
        {/* Status indicator */}
        <div className="text-center">
          {status === 'connecting' && (
            <div className="flex items-center justify-center gap-2 text-white/60">
              <div className="w-2 h-2 bg-yellow-400 rounded-full animate-pulse" />
              Connecting...
            </div>
          )}
          {status === 'waiting' && (
            <div className="flex items-center justify-center gap-2 text-white/60">
              <div className="w-2 h-2 bg-primary rounded-full animate-pulse" />
              Searching for opponent...
            </div>
          )}
          {status === 'matched' && (
            <div className="flex items-center justify-center gap-2 text-success">
              <div className="w-2 h-2 bg-success rounded-full" />
              Opponent found!
            </div>
          )}
          {status === 'playing' && !result && (
            <div className="flex items-center justify-center gap-2 text-primary">
              <div className="w-2 h-2 bg-primary rounded-full animate-pulse" />
              {waiting ? 'Waiting for opponent...' : 'Make your move!'}
            </div>
          )}
        </div>

        {/* Result display */}
        {result && (
          <div className="text-center space-y-4 animate-slideUp">
            {/* Battle display */}
            {result.payload?.details && (
              <div className="flex items-center justify-center gap-4 py-4">
                <div className="text-center">
                  <div className="text-5xl mb-2">
                    {getMoveIcon(result.payload.details.yourMove || selectedMove)}
                  </div>
                  <div className="text-sm text-white/60">You</div>
                </div>
                <div className="text-2xl text-white/40">VS</div>
                <div className="text-center">
                  <div className="text-5xl mb-2">
                    {getMoveIcon(result.payload.details.opponentMove)}
                  </div>
                  <div className="text-sm text-white/60">Opponent</div>
                </div>
              </div>
            )}

            <div className={`text-2xl font-bold ${getResultColor()}`}>
              {getResultText()}
            </div>

            {result.payload?.reason === 'opponent_left' && (
              <div className="text-white/60">Opponent left the game</div>
            )}
          </div>
        )}

        {/* Move selection */}
        {(status === 'playing' || status === 'matched') && !result && !selectedMove && (
          <div className="space-y-2">
            <label className="text-sm text-white/60 text-center block">Choose your move</label>
            <div className="grid grid-cols-3 gap-3">
              {MOVES.map((move) => (
                <button
                  key={move.id}
                  onClick={() => handleMove(move.id)}
                  disabled={waiting}
                  className="flex flex-col items-center gap-2 p-4 rounded-xl bg-white/10 hover:bg-white/20 transition-colors disabled:opacity-50"
                >
                  <span className="text-4xl">{move.icon}</span>
                  <span className="text-sm">{move.label}</span>
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Selected move display while waiting */}
        {waiting && !result && (
          <div className="text-center py-8">
            <div className="text-6xl mb-4 animate-pulse-custom">
              {getMoveIcon(selectedMove)}
            </div>
            <p className="text-white/60">Waiting for opponent's move...</p>
          </div>
        )}

        {/* Searching animation */}
        {(status === 'waiting' || status === 'connecting') && (
          <div className="flex justify-center py-8">
            <div className="text-6xl animate-pulse-custom">⚔️</div>
          </div>
        )}

        {/* Actions */}
        <div className="flex gap-3">
          {result ? (
            <>
              <Button variant="secondary" onClick={onClose} className="flex-1">
                Close
              </Button>
              <Button onClick={handlePlayAgain} className="flex-1">
                Play Again
              </Button>
            </>
          ) : (
            <Button variant="secondary" onClick={onClose} className="w-full">
              Cancel
            </Button>
          )}
        </div>
      </div>
    </Modal>
  )
}
