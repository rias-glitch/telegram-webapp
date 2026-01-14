import { useState, useMemo } from 'react'
import { Modal } from '../ui/Overlay'
import { Button, Input } from '../ui'
import { playDice } from '../../api/games'

const BET_PRESETS = [10, 50, 100, 500]

const DICE_FACES = ['⚀', '⚁', '⚂', '⚃', '⚄', '⚅']

export function DiceGame({ user, onClose, onResult }) {
  const [bet, setBet] = useState(10)
  const [target, setTarget] = useState(1)
  const [loading, setLoading] = useState(false)
  const [rolling, setRolling] = useState(false)
  const [result, setResult] = useState(null)
  const [displayValue, setDisplayValue] = useState(1)

  // Fixed values for 1-6 dice
  const winChance = 16.67 // 1 out of 6
  const multiplier = 5.5  // Fixed multiplier

  const handleRoll = async () => {
    if (bet <= 0 || bet > (user?.gems || 0)) return

    try {
      setLoading(true)
      setRolling(true)
      setResult(null)

      // Animate dice rolling (cycle through faces)
      const rollInterval = setInterval(() => {
        setDisplayValue(Math.floor(Math.random() * 6) + 1)
      }, 100)

      const response = await playDice(bet, target)

      // Stop animation and show result
      setTimeout(() => {
        clearInterval(rollInterval)
        setDisplayValue(response.result)
        setRolling(false)
        setResult(response)
        onResult(response.gems)
      }, 1500)
    } catch (err) {
      setRolling(false)
      setResult({ error: err.message })
    } finally {
      setLoading(false)
    }
  }

  const playAgain = () => {
    setResult(null)
  }

  return (
    <Modal isOpen={true} onClose={onClose} title="Dice">
      <div className="space-y-6">
        {/* Dice display */}
        <div className="flex justify-center py-6">
          <div className={`text-9xl transition-all duration-300 ${
            rolling ? 'animate-spin' :
            result ? (result.won ? 'text-success scale-110' : 'text-danger scale-110') : 'text-white'
          }`}>
            {DICE_FACES[displayValue - 1]}
          </div>
        </div>

        {/* Result */}
        {result && !result.error && (
          <div className="text-center space-y-2">
            <div className={`text-2xl font-bold ${result.won ? 'text-success' : 'text-danger'}`}>
              {result.won ? 'YOU WON!' : 'YOU LOST'}
            </div>
            <div className="text-white/60">
              {result.won ? `+${result.win_amount}` : `-${bet}`} gems
            </div>
            <div className="text-sm text-white/40">
              Your pick: {target} → Rolled: {result.result}
            </div>
          </div>
        )}

        {result?.error && (
          <div className="text-center text-danger">{result.error}</div>
        )}

        {/* Game controls */}
        {!result && (
          <>
            {/* Pick your number */}
            <div className="space-y-2">
              <label className="text-sm text-white/60">Pick your number (1-6)</label>
              <div className="grid grid-cols-6 gap-2">
                {[1, 2, 3, 4, 5, 6].map((num) => (
                  <button
                    key={num}
                    onClick={() => setTarget(num)}
                    className={`aspect-square rounded-xl font-bold text-3xl transition-all ${
                      target === num
                        ? 'bg-primary text-white scale-110 shadow-lg'
                        : 'bg-white/10 text-white/60 hover:bg-white/20 hover:scale-105'
                    }`}
                  >
                    {DICE_FACES[num - 1]}
                  </button>
                ))}
              </div>
              <div className="text-center text-sm text-white/40">
                Pick a number and roll the dice!
              </div>
            </div>

            {/* Stats */}
            <div className="grid grid-cols-2 gap-4 text-center">
              <div className="bg-white/5 rounded-xl p-3">
                <div className="text-white/60 text-sm">Win Chance</div>
                <div className="text-xl font-bold text-success">{winChance.toFixed(2)}%</div>
              </div>
              <div className="bg-white/5 rounded-xl p-3">
                <div className="text-white/60 text-sm">Multiplier</div>
                <div className="text-xl font-bold text-primary">{multiplier}x</div>
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

            <div className="flex justify-between text-sm text-white/60">
              <span>Balance: {user?.gems?.toLocaleString() || 0}</span>
              <span>Potential win: {Math.floor(bet * multiplier)}</span>
            </div>
          </>
        )}

        {/* Actions */}
        <div className="flex gap-3">
          {result ? (
            <>
              <Button variant="secondary" onClick={onClose} className="flex-1">
                Close
              </Button>
              <Button onClick={playAgain} className="flex-1">
                Roll Again
              </Button>
            </>
          ) : (
            <>
              <Button variant="secondary" onClick={onClose} className="flex-1">
                Cancel
              </Button>
              <Button
                onClick={handleRoll}
                disabled={loading || rolling || bet <= 0 || bet > (user?.gems || 0)}
                className="flex-1"
              >
                {rolling ? 'Rolling...' : `Roll (${bet})`}
              </Button>
            </>
          )}
        </div>
      </div>
    </Modal>
  )
}
