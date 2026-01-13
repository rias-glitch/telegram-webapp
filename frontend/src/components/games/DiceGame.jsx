import { useState, useMemo } from 'react'
import { Modal } from '../ui/Overlay'
import { Button, Input } from '../ui'
import { playDice } from '../../api/games'

const BET_PRESETS = [10, 50, 100, 500]

export function DiceGame({ user, onClose, onResult }) {
  const [bet, setBet] = useState(10)
  const [target, setTarget] = useState(50)
  const [rollOver, setRollOver] = useState(true)
  const [loading, setLoading] = useState(false)
  const [rolling, setRolling] = useState(false)
  const [result, setResult] = useState(null)
  const [displayValue, setDisplayValue] = useState(0)

  // Calculate win chance and multiplier
  const { winChance, multiplier } = useMemo(() => {
    let chance
    if (rollOver) {
      chance = 99.99 - target
    } else {
      chance = target
    }
    chance = Math.max(0.01, Math.min(98.99, chance))
    const mult = Math.floor((100 / chance) * 100) / 100
    return { winChance: chance, multiplier: mult }
  }, [target, rollOver])

  const handleRoll = async () => {
    if (bet <= 0 || bet > (user?.gems || 0)) return

    try {
      setLoading(true)
      setRolling(true)
      setResult(null)

      // Animate dice rolling
      const rollInterval = setInterval(() => {
        setDisplayValue(Math.random() * 100)
      }, 50)

      const response = await playDice(bet, target, rollOver)

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
          <div className={`text-6xl font-mono font-bold transition-all ${
            rolling ? 'animate-pulse text-white/60' :
            result ? (result.won ? 'text-success' : 'text-danger') : 'text-white'
          }`}>
            {displayValue.toFixed(2)}
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
              Roll {rollOver ? 'over' : 'under'} {target} → Got {result.result.toFixed(2)}
            </div>
          </div>
        )}

        {result?.error && (
          <div className="text-center text-danger">{result.error}</div>
        )}

        {/* Game controls */}
        {!result && (
          <>
            {/* Roll over/under toggle */}
            <div className="flex gap-2">
              <button
                onClick={() => setRollOver(false)}
                className={`flex-1 py-2 rounded-lg font-medium transition-colors ${
                  !rollOver ? 'bg-primary text-white' : 'bg-white/10 text-white/60'
                }`}
              >
                Roll Under
              </button>
              <button
                onClick={() => setRollOver(true)}
                className={`flex-1 py-2 rounded-lg font-medium transition-colors ${
                  rollOver ? 'bg-primary text-white' : 'bg-white/10 text-white/60'
                }`}
              >
                Roll Over
              </button>
            </div>

            {/* Target slider */}
            <div className="space-y-2">
              <div className="flex justify-between text-sm">
                <span className="text-white/60">Target: {target.toFixed(2)}</span>
                <span className="text-white/60">
                  {rollOver ? `> ${target}` : `< ${target}`}
                </span>
              </div>
              <input
                type="range"
                min="1"
                max="98.99"
                step="0.01"
                value={target}
                onChange={(e) => setTarget(parseFloat(e.target.value))}
                className="w-full h-2 bg-white/20 rounded-lg appearance-none cursor-pointer accent-primary"
              />
              {/* Visual indicator */}
              <div className="h-2 bg-white/10 rounded-full overflow-hidden relative">
                <div
                  className={`absolute h-full ${rollOver ? 'bg-success right-0' : 'bg-success left-0'}`}
                  style={{ width: `${winChance}%` }}
                />
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
                <div className="text-xl font-bold text-primary">{multiplier.toFixed(2)}x</div>
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
