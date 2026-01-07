import { useState, useEffect } from 'react'
import { Modal } from '../ui/Overlay'
import { Button } from '../ui'
import { useWebSocket } from '../../hooks/useWebSocket'

const GRID_SIZE = 12
const MINES_COUNT = 4

export function PvPMinesGame({ user, onClose, onResult }) {
  const {
    status,
    opponent,
    roomId,
    gameState,
    result,
    connect,
    send,
    disconnect,
  } = useWebSocket('mines')

  const [phase, setPhase] = useState('connecting') // connecting, setup, playing, finished
  const [selectedMines, setSelectedMines] = useState([])
  const [setupSubmitted, setSetupSubmitted] = useState(false)
  const [selectedCell, setSelectedCell] = useState(null)
  const [waitingForOpponent, setWaitingForOpponent] = useState(false)
  const [round, setRound] = useState(1)

  useEffect(() => {
    connect(0)
    return () => disconnect()
  }, [])

  useEffect(() => {
    if (status === 'playing' || status === 'matched') {
      if (!setupSubmitted) {
        setPhase('setup')
      } else {
        setPhase('playing')
      }
    } else if (status === 'waiting' || status === 'connecting') {
      setPhase('connecting')
    }
  }, [status, setupSubmitted])

  useEffect(() => {
    if (result) {
      setPhase('finished')
      setWaitingForOpponent(false)
    }
  }, [result])

  // Handle setup_complete message
  useEffect(() => {
    if (gameState?.type === 'setup_complete') {
      setPhase('playing')
    }
    if (gameState?.round) {
      setRound(gameState.round)
    }
  }, [gameState])

  const toggleMine = (index) => {
    if (setupSubmitted) return

    const cellNum = index + 1
    if (selectedMines.includes(cellNum)) {
      setSelectedMines(selectedMines.filter(m => m !== cellNum))
    } else if (selectedMines.length < MINES_COUNT) {
      setSelectedMines([...selectedMines, cellNum])
    }
  }

  const submitSetup = () => {
    if (selectedMines.length !== MINES_COUNT) return

    send({ type: 'setup', value: selectedMines })
    setSetupSubmitted(true)
    setPhase('playing')
  }

  const selectCell = (index) => {
    if (waitingForOpponent) return

    const cellNum = index + 1
    setSelectedCell(cellNum)
    setWaitingForOpponent(true)
    send({ type: 'move', value: cellNum })
  }

  const handlePlayAgain = () => {
    setPhase('connecting')
    setSelectedMines([])
    setSetupSubmitted(false)
    setSelectedCell(null)
    setWaitingForOpponent(false)
    setRound(1)
    disconnect()
    setTimeout(() => connect(0), 100)
  }

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
    <Modal isOpen={true} onClose={onClose} title="PvP Mines">
      <div className="space-y-6">
        {/* Status */}
        <div className="text-center">
          {phase === 'connecting' && (
            <div className="flex items-center justify-center gap-2 text-white/60">
              <div className="w-2 h-2 bg-primary rounded-full animate-pulse" />
              {status === 'waiting' ? 'Searching for opponent...' : 'Connecting...'}
            </div>
          )}
          {phase === 'setup' && !setupSubmitted && (
            <div className="text-primary">
              Place {MINES_COUNT} mines on your field ({selectedMines.length}/{MINES_COUNT})
            </div>
          )}
          {phase === 'setup' && setupSubmitted && (
            <div className="flex items-center justify-center gap-2 text-white/60">
              <div className="w-2 h-2 bg-yellow-400 rounded-full animate-pulse" />
              Waiting for opponent to place mines...
            </div>
          )}
          {phase === 'playing' && !waitingForOpponent && (
            <div className="text-primary">
              Round {round}/5 - Pick a cell on opponent's field!
            </div>
          )}
          {phase === 'playing' && waitingForOpponent && (
            <div className="flex items-center justify-center gap-2 text-white/60">
              <div className="w-2 h-2 bg-yellow-400 rounded-full animate-pulse" />
              Waiting for opponent's move...
            </div>
          )}
        </div>

        {/* Searching animation */}
        {phase === 'connecting' && (
          <div className="flex justify-center py-8">
            <div className="text-6xl animate-pulse-custom">💣</div>
          </div>
        )}

        {/* Setup Grid - place your mines */}
        {phase === 'setup' && !setupSubmitted && (
          <div className="space-y-3">
            <div className="text-center text-sm text-white/60">Your field - tap to place mines</div>
            <div className="grid grid-cols-4 gap-2">
              {Array.from({ length: GRID_SIZE }).map((_, index) => {
                const cellNum = index + 1
                const hasMine = selectedMines.includes(cellNum)
                return (
                  <button
                    key={index}
                    onClick={() => toggleMine(index)}
                    className={`aspect-square rounded-xl border text-2xl font-bold transition-all ${
                      hasMine
                        ? 'bg-danger/30 border-danger text-danger'
                        : 'bg-white/10 border-white/20 hover:bg-white/20'
                    }`}
                  >
                    {hasMine ? '💣' : ''}
                  </button>
                )
              })}
            </div>
            <Button
              onClick={submitSetup}
              disabled={selectedMines.length !== MINES_COUNT}
              className="w-full"
            >
              Confirm Mines ({selectedMines.length}/{MINES_COUNT})
            </Button>
          </div>
        )}

        {/* Waiting after setup */}
        {phase === 'setup' && setupSubmitted && (
          <div className="flex justify-center py-8">
            <div className="text-6xl animate-pulse-custom">⏳</div>
          </div>
        )}

        {/* Playing Grid - opponent's field */}
        {phase === 'playing' && (
          <div className="space-y-3">
            <div className="text-center text-sm text-white/60">Opponent's field - find safe cells!</div>
            <div className="grid grid-cols-4 gap-2">
              {Array.from({ length: GRID_SIZE }).map((_, index) => {
                const cellNum = index + 1
                const isSelected = selectedCell === cellNum
                return (
                  <button
                    key={index}
                    onClick={() => selectCell(index)}
                    disabled={waitingForOpponent}
                    className={`aspect-square rounded-xl border text-2xl font-bold transition-all ${
                      isSelected
                        ? 'bg-primary/30 border-primary'
                        : 'bg-white/10 border-white/20 hover:bg-white/20'
                    } disabled:opacity-50 disabled:cursor-not-allowed`}
                  >
                    {isSelected ? '👆' : '?'}
                  </button>
                )
              })}
            </div>
          </div>
        )}

        {/* Result */}
        {phase === 'finished' && result && (
          <div className="text-center space-y-4 animate-slideUp">
            <div className="text-6xl mb-4">
              {result.payload?.you === 'win' ? '🏆' : result.payload?.you === 'lose' ? '💥' : '🤝'}
            </div>
            <div className={`text-2xl font-bold ${getResultColor()}`}>
              {getResultText()}
            </div>
            {result.payload?.reason && (
              <div className="text-white/60 text-sm">
                {result.payload.reason === 'opponent_hit_mine' && 'Opponent hit a mine!'}
                {result.payload.reason === 'you_hit_mine' && 'You hit a mine!'}
                {result.payload.reason === 'opponent_left' && 'Opponent left the game'}
                {result.payload.reason === 'draw' && '5 rounds completed - Draw!'}
              </div>
            )}
          </div>
        )}

        {/* Actions */}
        <div className="flex gap-3">
          {phase === 'finished' ? (
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
