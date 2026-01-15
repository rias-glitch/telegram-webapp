import { useState } from 'react'
import { Modal } from '../ui/Overlay'
import { Button } from '../ui'
import { Input } from '../ui'
import { playMines } from '../../api/games'

const BET_PRESETS = [10, 50, 100, 500]
const GRID_SIZE = 12

export function MinesGame({ user, onClose, onResult, embedded = false }) {
  const [bet, setBet] = useState(100)
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState(null)
  const [selectedCell, setSelectedCell] = useState(null)

  const handlePlay = async (cellIndex) => {
    if (bet <= 0 || bet > (user?.gems || 0)) return

    try {
      setLoading(true)
      setSelectedCell(cellIndex)
      setResult(null)

      const response = await playMines(bet, cellIndex)

      // Animation delay
      setTimeout(() => {
        setResult(response)
        onResult(response.gems)
      }, 500)
    } catch (err) {
      setResult({ error: err.message })
    } finally {
      setLoading(false)
    }
  }

  const playAgain = () => {
    setResult(null)
    setSelectedCell(null)
  }

  const getCellContent = (index) => {
    if (!result) return '?'
    if (index === selectedCell) {
      return result.win ? '💎' : '💥'
    }
    return '?'
  }

  const getCellStyle = (index) => {
    if (!result) {
      return 'bg-white/10 hover:bg-white/20 cursor-pointer'
    }
    if (index === selectedCell) {
      return result.win ? 'bg-success/30 border-success' : 'bg-danger/30 border-danger'
    }
    return 'bg-white/5'
  }

  const gameContent = (
    <div className="space-y-6">
        {/* Result */}
        {result && !result.error && (
          <div className="text-center space-y-2">
            <div className={`text-2xl font-bold ${result.win ? 'text-success' : 'text-danger'}`}>
              {result.win ? 'SAFE!' : 'BOOM!'}
            </div>
            <div className="text-white/60">
              {result.win ? `+${result.awarded} gems` : `-${bet} gems`}
            </div>
          </div>
        )}

        {result?.error && (
          <div className="text-center text-danger">{result.error}</div>
        )}

        {/* Grid */}
        <div className="grid grid-cols-4 gap-2">
          {Array.from({ length: GRID_SIZE }).map((_, index) => (
            <button
              key={index}
              onClick={() => !result && !loading && handlePlay(index + 1)}
              disabled={loading || !!result || bet <= 0 || bet > (user?.gems || 0)}
              className={`aspect-square rounded-xl border border-white/20 text-2xl font-bold transition-all ${getCellStyle(index + 1)} disabled:cursor-not-allowed`}
            >
              {getCellContent(index + 1)}
            </button>
          ))}
        </div>

        <div className="text-center text-white/40 text-xs">
          4 mines hidden in 12 cells. Pick a safe one!
        </div>

        {/* Bet controls */}
        {!result && (
          <>
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
          </>
        )}

      {/* Actions */}
      <div className="flex gap-3">
        {result ? (
          <>
            <Button variant="secondary" onClick={onClose} className="flex-1">
              {embedded ? 'Back' : 'Close'}
            </Button>
            <Button onClick={playAgain} className="flex-1">
              Play Again
            </Button>
          </>
        ) : !embedded ? (
          <Button variant="secondary" onClick={onClose} className="w-full">
            Cancel
          </Button>
        ) : null}
      </div>
    </div>
  )

  if (embedded) {
    return gameContent
  }

  return (
    <Modal isOpen={true} onClose={onClose} title="Mines">
      {gameContent}
    </Modal>
  )
}
