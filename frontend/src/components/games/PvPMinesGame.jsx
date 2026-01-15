import { useState, useEffect, useRef } from 'react'
import { Modal } from '../ui/Overlay'
import { Button } from '../ui'
import { useWebSocket } from '../../hooks/useWebSocket'

const GRID_SIZE = 12
const MINES_COUNT = 4
const TURN_TIMEOUT = 10 // seconds

export function PvPMinesGame({ user, onClose, onResult, embedded = false, initialBet = 0, initialCurrency = 'gems' }) {
  const {
    status,
    opponent,
    roomId,
    gameState,
    result,
    roundResult,
    moveHistory,
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
  const [timer, setTimer] = useState(TURN_TIMEOUT)
  const [openedCells, setOpenedCells] = useState([]) // {cell, hitMine}[]
  const [lastRoundAnimation, setLastRoundAnimation] = useState(null) // 'safe' | 'explode' | null
  const timerRef = useRef(null)

  useEffect(() => {
    connect(initialBet, initialCurrency)
    return () => disconnect()
  }, [initialBet, initialCurrency])

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
      stopTimer()
    }
  }, [result])

  // Handle round result - update opened cells and prepare for next round
  useEffect(() => {
    if (roundResult && roundResult.your_move) {
      const { your_move, your_hit } = roundResult

      console.log('PvPMinesGame: received round_result', roundResult)

      // Add to opened cells (avoid duplicates)
      setOpenedCells(prev => {
        const alreadyExists = prev.some(c => c.cell === your_move)
        if (alreadyExists) {
          console.log('PvPMinesGame: cell already opened, skipping', your_move)
          return prev
        }
        return [...prev, { cell: your_move, hitMine: your_hit }]
      })

      // Show animation
      setLastRoundAnimation(your_hit ? 'explode' : 'safe')
      setTimeout(() => setLastRoundAnimation(null), 1500)

      // Update round from next_round if available, otherwise increment
      if (roundResult.next_round) {
        setRound(roundResult.next_round)
      } else if (roundResult.round) {
        setRound(roundResult.round + 1)
      }

      // Reset for next round
      console.log('PvPMinesGame: resetting for next round')
      setSelectedCell(null)
      setWaitingForOpponent(false)
      startTimer()
    }
  }, [roundResult])

  // Handle setup_complete message and new rounds
  useEffect(() => {
    if (gameState?.type === 'setup_complete') {
      setPhase('playing')
      startTimer()
    }
    if (gameState?.type === 'round_draw' || gameState?.type === 'start') {
      // New round started - reset for new move
      setSelectedCell(null)
      setWaitingForOpponent(false)
      startTimer()
    }
  }, [gameState])

  // Timer logic
  const startTimer = () => {
    stopTimer()
    setTimer(TURN_TIMEOUT)
    timerRef.current = setInterval(() => {
      setTimer(prev => {
        if (prev <= 1) {
          stopTimer()
          return 0
        }
        return prev - 1
      })
    }, 1000)
  }

  const stopTimer = () => {
    if (timerRef.current) {
      clearInterval(timerRef.current)
      timerRef.current = null
    }
  }

  useEffect(() => {
    return () => stopTimer()
  }, [])

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

    // Check if cell was already opened
    if (openedCells.some(c => c.cell === cellNum)) return

    setSelectedCell(cellNum)
    setWaitingForOpponent(true)
    stopTimer()
    send({ type: 'move', value: cellNum })
  }

  const handlePlayAgain = () => {
    setPhase('connecting')
    setSelectedMines([])
    setSetupSubmitted(false)
    setSelectedCell(null)
    setWaitingForOpponent(false)
    setRound(1)
    setOpenedCells([])
    setTimer(TURN_TIMEOUT)
    stopTimer()
    disconnect()
    setTimeout(() => connect(initialBet, initialCurrency), 100)
  }

  const currencyIcon = initialCurrency === 'coins' ? '🪙' : '💎'

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
    if (you === 'win') return 'text-green-400'
    if (you === 'lose') return 'text-red-400'
    return 'text-yellow-400'
  }

  const getCellState = (index) => {
    const cellNum = index + 1
    const opened = openedCells.find(c => c.cell === cellNum)
    if (opened) {
      return opened.hitMine ? 'exploded' : 'safe'
    }
    if (selectedCell === cellNum) return 'selected'
    return 'unknown'
  }

  const getOpponentName = () => {
    if (opponent?.first_name) return opponent.first_name
    if (opponent?.username) return `@${opponent.username}`
    if (opponent?.id) return `Player #${opponent.id}`
    return 'Opponent'
  }

  // Timer progress percentage
  const timerProgress = (timer / TURN_TIMEOUT) * 100
  const timerColor = timer > 5 ? 'bg-green-500' : timer > 2 ? 'bg-yellow-500' : 'bg-red-500'

  const gameContent = (
    <div className="space-y-4">
        {/* Opponent info */}
        {opponent && phase !== 'connecting' && (
          <div className="flex items-center justify-center gap-3 p-3 bg-white/5 rounded-xl">
            <div className="w-10 h-10 rounded-full bg-gradient-to-br from-purple-500 to-pink-500 flex items-center justify-center text-lg font-bold">
              {getOpponentName().charAt(0).toUpperCase()}
            </div>
            <div>
              <div className="text-sm text-white/60">Playing against</div>
              <div className="font-semibold">{getOpponentName()}</div>
            </div>
          </div>
        )}

        {/* Timer bar */}
        {phase === 'playing' && !waitingForOpponent && !result && (
          <div className="space-y-1">
            <div className="flex justify-between text-sm">
              <span className="text-white/60">Time remaining</span>
              <span className={timer <= 3 ? 'text-red-400 font-bold animate-pulse' : 'text-white/80'}>
                {timer}s
              </span>
            </div>
            <div className="h-2 bg-white/10 rounded-full overflow-hidden">
              <div
                className={`h-full ${timerColor} transition-all duration-1000 ease-linear`}
                style={{ width: `${timerProgress}%` }}
              />
            </div>
          </div>
        )}

        {/* Status */}
        <div className="text-center">
          {phase === 'connecting' && (
            <div className="flex items-center justify-center gap-2 text-white/60">
              <div className="w-2 h-2 bg-primary rounded-full animate-pulse" />
              {status === 'waiting' ? 'Searching for opponent...' : 'Connecting...'}
            </div>
          )}
          {phase === 'setup' && !setupSubmitted && (
            <div className="text-primary font-medium">
              Place {MINES_COUNT} mines on your field ({selectedMines.length}/{MINES_COUNT})
            </div>
          )}
          {phase === 'setup' && setupSubmitted && (
            <div className="flex items-center justify-center gap-2 text-white/60">
              <div className="w-2 h-2 bg-yellow-400 rounded-full animate-pulse" />
              Waiting for opponent to place mines...
            </div>
          )}
          {phase === 'playing' && !waitingForOpponent && !result && (
            <div className="text-primary font-medium">
              Round {round}/5 - Pick a cell!
            </div>
          )}
          {phase === 'playing' && waitingForOpponent && !result && (
            <div className="flex items-center justify-center gap-2 text-white/60">
              <div className="w-2 h-2 bg-yellow-400 rounded-full animate-pulse" />
              Waiting for {getOpponentName()}'s move...
            </div>
          )}
        </div>

        {/* Round animation overlay */}
        {lastRoundAnimation && (
          <div className="flex justify-center py-2">
            {lastRoundAnimation === 'safe' ? (
              <div className="text-4xl animate-bounce text-green-400">
                Safe! +1
              </div>
            ) : (
              <div className="text-4xl animate-pulse text-red-400">
                BOOM!
              </div>
            )}
          </div>
        )}

        {/* Searching animation */}
        {phase === 'connecting' && (
          <div className="flex justify-center py-8">
            <div className="text-6xl animate-pulse-custom">
              <span className="inline-block animate-bounce">💣</span>
            </div>
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
                    className={`aspect-square rounded-xl border-2 text-2xl font-bold transition-all transform active:scale-95 ${
                      hasMine
                        ? 'bg-red-500/30 border-red-500 text-red-400 shadow-lg shadow-red-500/20'
                        : 'bg-white/5 border-white/20 hover:bg-white/10 hover:border-white/30'
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
            <div className="text-6xl animate-spin-slow">
              <span>⏳</span>
            </div>
          </div>
        )}

        {/* Playing Grid - opponent's field */}
        {phase === 'playing' && !result && (
          <div className="space-y-3">
            <div className="text-center text-sm text-white/60">
              {getOpponentName()}'s field - find safe cells!
            </div>
            <div className="grid grid-cols-4 gap-2">
              {Array.from({ length: GRID_SIZE }).map((_, index) => {
                const cellNum = index + 1
                const state = getCellState(index)
                const isDisabled = waitingForOpponent || state !== 'unknown'

                return (
                  <button
                    key={index}
                    onClick={() => selectCell(index)}
                    disabled={isDisabled}
                    className={`aspect-square rounded-xl border-2 text-2xl font-bold transition-all transform ${
                      state === 'exploded'
                        ? 'bg-red-500/40 border-red-500 text-red-400 animate-pulse cursor-not-allowed'
                        : state === 'safe'
                        ? 'bg-green-500/40 border-green-500 text-green-400 cursor-not-allowed'
                        : state === 'selected'
                        ? 'bg-primary/30 border-primary animate-pulse'
                        : 'bg-white/5 border-white/20 hover:bg-white/10 hover:border-primary/50 active:scale-95'
                    } ${isDisabled && state === 'unknown' ? 'opacity-50 cursor-not-allowed' : ''}`}
                  >
                    {state === 'exploded' ? '💥' :
                     state === 'safe' ? '✓' :
                     state === 'selected' ? '👆' : '?'}
                  </button>
                )
              })}
            </div>

            {/* Progress indicator */}
            <div className="flex justify-center gap-2 mt-4">
              {[1, 2, 3, 4, 5].map(r => (
                <div
                  key={r}
                  className={`w-3 h-3 rounded-full transition-all ${
                    r < round
                      ? 'bg-green-500'
                      : r === round
                      ? 'bg-primary animate-pulse'
                      : 'bg-white/20'
                  }`}
                />
              ))}
            </div>
          </div>
        )}

        {/* Result */}
        {phase === 'finished' && result && (
          <div className="text-center space-y-4 animate-slideUp">
            <div className="text-7xl mb-4">
              {result.payload?.you === 'win' ? '🏆' : result.payload?.you === 'lose' ? '💀' : '🤝'}
            </div>
            <div className={`text-3xl font-bold ${getResultColor()}`}>
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

            {/* Show final stats */}
            <div className="bg-white/5 rounded-xl p-4 mt-4">
              <div className="text-sm text-white/60 mb-2">Your moves</div>
              <div className="flex justify-center gap-2 flex-wrap">
                {openedCells.map((cell, i) => (
                  <div
                    key={i}
                    className={`w-8 h-8 rounded-lg flex items-center justify-center text-sm ${
                      cell.hitMine ? 'bg-red-500/30 text-red-400' : 'bg-green-500/30 text-green-400'
                    }`}
                  >
                    {cell.cell}
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

      {/* Bet info */}
      {initialBet > 0 && (
        <div className="text-center text-white/60 text-sm">
          Bet: {initialBet} {currencyIcon}
        </div>
      )}

      {/* Actions */}
      <div className="flex gap-3 pt-2">
        {phase === 'finished' ? (
          <>
            <Button variant="secondary" onClick={onClose} className="flex-1">
              {embedded ? 'Back' : 'Close'}
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
  )

  if (embedded) {
    return gameContent
  }

  return (
    <Modal isOpen={true} onClose={onClose} title="PvP Mines">
      {gameContent}
    </Modal>
  )
}
