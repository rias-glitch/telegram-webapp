import { useState, useEffect } from 'react'
import { Modal } from '../ui/Overlay'
import { Button, Input } from '../ui'
import { startMinesPro, revealMinesPro, cashoutMinesPro, getMinesProState } from '../../api/games'

const BET_PRESETS = [10, 50, 100, 500]
const MINES_PRESETS = [3, 5, 10, 15]
const BOARD_SIZE = 25

export function MinesProGame({ user, onClose, onResult }) {
  const [bet, setBet] = useState(10)
  const [minesCount, setMinesCount] = useState(5)
  const [loading, setLoading] = useState(false)
  const [gameState, setGameState] = useState(null)
  const [revealedCells, setRevealedCells] = useState([])
  const [mines, setMines] = useState([])
  const [gameOver, setGameOver] = useState(false)
  const [won, setWon] = useState(false)

  // Check for active game on mount
  useEffect(() => {
    getMinesProState().then(state => {
      if (state.active) {
        setGameState(state)
        setRevealedCells(state.revealed || [])
      }
    }).catch(() => {})
  }, [])

  const handleStart = async () => {
    if (bet <= 0 || bet > (user?.gems || 0)) return

    try {
      setLoading(true)
      const state = await startMinesPro(bet, minesCount)
      setGameState(state)
      setRevealedCells([])
      setMines([])
      setGameOver(false)
      setWon(false)
      onResult(state.gems)
    } catch (err) {
      alert(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleReveal = async (cell) => {
    if (!gameState || gameOver || revealedCells.includes(cell)) return

    try {
      setLoading(true)
      const result = await revealMinesPro(cell)

      setRevealedCells(result.revealed || [])
      setGameState(result)

      if (result.hit_mine) {
        setMines(result.mines || [])
        setGameOver(true)
        setWon(false)
        onResult(result.gems)
      } else if (result.status === 'cashed_out') {
        setWon(true)
        setGameOver(true)
        onResult(result.gems)
      }
    } catch (err) {
      alert(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleCashout = async () => {
    if (!gameState || gameOver) return

    try {
      setLoading(true)
      const result = await cashoutMinesPro()
      setGameState(result)
      setMines(result.mines || [])
      setGameOver(true)
      setWon(true)
      onResult(result.gems)
    } catch (err) {
      alert(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleNewGame = () => {
    setGameState(null)
    setRevealedCells([])
    setMines([])
    setGameOver(false)
    setWon(false)
  }

  const isActive = gameState && !gameOver

  return (
    <Modal isOpen={true} onClose={onClose} title="Mines Pro">
      <div className="space-y-4">
        {/* Game board */}
        {gameState && (
          <>
            {/* Stats bar */}
            <div className="flex justify-between items-center bg-white/5 rounded-xl p-3">
              <div>
                <div className="text-xs text-white/60">Bet</div>
                <div className="font-bold">{gameState.bet}</div>
              </div>
              <div>
                <div className="text-xs text-white/60">Mines</div>
                <div className="font-bold">{gameState.mines_count}</div>
              </div>
              <div>
                <div className="text-xs text-white/60">Multiplier</div>
                <div className="font-bold text-primary">{gameState.current_multiplier?.toFixed(2)}x</div>
              </div>
              <div>
                <div className="text-xs text-white/60">Win</div>
                <div className="font-bold text-success">{gameState.potential_win}</div>
              </div>
            </div>

            {/* Board */}
            <div className="grid grid-cols-5 gap-2">
              {Array.from({ length: BOARD_SIZE }).map((_, i) => {
                const isRevealed = revealedCells.includes(i)
                const isMine = mines.includes(i)
                const isGem = gameOver && !isMine
                const isClickable = isActive && !isRevealed

                return (
                  <button
                    key={i}
                    onClick={() => isClickable && handleReveal(i)}
                    disabled={!isClickable || loading}
                    className={`
                      aspect-square rounded-lg flex items-center justify-center text-2xl
                      transition-all transform
                      ${isRevealed
                        ? isMine
                          ? 'bg-danger scale-95'
                          : 'bg-success scale-95'
                        : gameOver
                          ? isMine
                            ? 'bg-danger/50'
                            : 'bg-success/30'
                          : isClickable
                            ? 'bg-white/10 hover:bg-white/20 hover:scale-105 cursor-pointer'
                            : 'bg-white/5'
                      }
                    `}
                  >
                    {isRevealed
                      ? (isMine ? '💣' : '💎')
                      : gameOver
                        ? (isMine ? '💣' : '💎')
                        : ''
                    }
                  </button>
                )
              })}
            </div>

            {/* Result */}
            {gameOver && (
              <div className={`text-center py-4 rounded-xl ${won ? 'bg-success/20' : 'bg-danger/20'}`}>
                <div className={`text-2xl font-bold ${won ? 'text-success' : 'text-danger'}`}>
                  {won ? 'CASHED OUT!' : 'BOOM!'}
                </div>
                <div className="text-white/60">
                  {won
                    ? `Won ${gameState.win_amount} gems (${gameState.current_multiplier?.toFixed(2)}x)`
                    : `Lost ${gameState.bet} gems`
                  }
                </div>
              </div>
            )}

            {/* Actions */}
            <div className="flex gap-3">
              {gameOver ? (
                <>
                  <Button variant="secondary" onClick={onClose} className="flex-1">
                    Close
                  </Button>
                  <Button onClick={handleNewGame} className="flex-1">
                    New Game
                  </Button>
                </>
              ) : (
                <>
                  <Button
                    variant="secondary"
                    onClick={onClose}
                    className="flex-1"
                  >
                    Exit
                  </Button>
                  <Button
                    onClick={handleCashout}
                    disabled={loading || revealedCells.length === 0}
                    className="flex-1 bg-success hover:bg-success/80"
                  >
                    Cash Out 💎{gameState.potential_win}
                  </Button>
                </>
              )}
            </div>
          </>
        )}

        {/* Setup screen */}
        {!gameState && (
          <>
            <div className="text-center py-8">
              <div className="text-6xl mb-4">💣</div>
              <p className="text-white/60">
                Reveal gems, avoid mines. Cash out anytime!
              </p>
            </div>

            {/* Mines count */}
            <div className="space-y-2">
              <label className="text-sm text-white/60">Number of mines</label>
              <div className="flex gap-2">
                {MINES_PRESETS.map((count) => (
                  <button
                    key={count}
                    onClick={() => setMinesCount(count)}
                    className={`flex-1 py-2 rounded-lg font-medium transition-colors ${
                      minesCount === count
                        ? 'bg-primary text-white'
                        : 'bg-white/10 text-white/60 hover:bg-white/20'
                    }`}
                  >
                    {count}
                  </button>
                ))}
              </div>
              <input
                type="range"
                min="1"
                max="24"
                value={minesCount}
                onChange={(e) => setMinesCount(parseInt(e.target.value))}
                className="w-full h-2 bg-white/20 rounded-lg appearance-none cursor-pointer accent-primary"
              />
              <div className="text-center text-sm text-white/40">
                {minesCount} mines = higher risk, higher reward
              </div>
            </div>

            {/* Bet controls */}
            <div className="space-y-2">
              <label className="text-sm text-white/60">Bet amount</label>
              <Input
                type="number"
                value={bet}
                onChange={(e) => setBet(Math.max(1, parseInt(e.target.value) || 0))}
                min={1}
                max={user?.gems || 0}
              />
              <div className="flex gap-2">
                {BET_PRESETS.map((preset) => (
                  <button
                    key={preset}
                    onClick={() => setBet(preset)}
                    className={`flex-1 py-1 rounded-lg text-sm transition-colors ${
                      bet === preset
                        ? 'bg-primary text-white'
                        : 'bg-white/10 text-white/60 hover:bg-white/20'
                    }`}
                  >
                    {preset}
                  </button>
                ))}
              </div>
            </div>

            <div className="text-center text-white/60 text-sm">
              Balance: {user?.gems?.toLocaleString() || 0} gems
            </div>

            {/* Actions */}
            <div className="flex gap-3">
              <Button variant="secondary" onClick={onClose} className="flex-1">
                Cancel
              </Button>
              <Button
                onClick={handleStart}
                disabled={loading || bet <= 0 || bet > (user?.gems || 0)}
                className="flex-1"
              >
                {loading ? 'Starting...' : 'Start Game'}
              </Button>
            </div>
          </>
        )}
      </div>
    </Modal>
  )
}
