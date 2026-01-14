import { useState, useMemo } from 'react'
import { Modal } from '../ui/Overlay'
import { Button, Input } from '../ui'
import { playDice } from '../../api/games'

const BET_PRESETS = [10, 50, 100, 500]

const DICE_FACES = ['⚀', '⚁', '⚂', '⚃', '⚄', '⚅']

// 3D Dice component
function Dice3D({ value, rolling }) {
  // Rotation angles for each face (1-6)
  const rotations = {
    1: 'rotateX(0deg) rotateY(0deg)',
    2: 'rotateX(0deg) rotateY(90deg)',
    3: 'rotateX(0deg) rotateY(180deg)',
    4: 'rotateX(0deg) rotateY(-90deg)',
    5: 'rotateX(90deg) rotateY(0deg)',
    6: 'rotateX(-90deg) rotateY(0deg)',
  }

  const rollAnimation = rolling ? 'rotateX(720deg) rotateY(720deg)' : rotations[value]

  return (
    <div className="perspective-1000">
      <div
        className={`dice-3d ${rolling ? 'rolling' : ''}`}
        style={{
          transform: rollAnimation,
          transition: rolling ? 'transform 1.5s cubic-bezier(0.34, 1.56, 0.64, 1)' : 'transform 0.3s ease',
        }}
      >
        {[1, 2, 3, 4, 5, 6].map((face) => (
          <div key={face} className={`dice-face dice-face-${face}`}>
            <div className="dice-dots">
              {Array.from({ length: face }).map((_, i) => (
                <div key={i} className="dice-dot" />
              ))}
            </div>
          </div>
        ))}
      </div>
      <style jsx>{`
        .perspective-1000 {
          perspective: 1000px;
          display: flex;
          justify-content: center;
          align-items: center;
          height: 150px;
        }

        .dice-3d {
          width: 100px;
          height: 100px;
          position: relative;
          transform-style: preserve-3d;
          transform-origin: center;
        }

        .dice-face {
          position: absolute;
          width: 100px;
          height: 100px;
          background: linear-gradient(145deg, #ffffff, #e0e0e0);
          border: 2px solid #333;
          border-radius: 12px;
          display: flex;
          align-items: center;
          justify-content: center;
          box-shadow: inset 0 0 10px rgba(0,0,0,0.1);
        }

        .dice-face-1 { transform: rotateY(0deg) translateZ(50px); }
        .dice-face-2 { transform: rotateY(90deg) translateZ(50px); }
        .dice-face-3 { transform: rotateY(180deg) translateZ(50px); }
        .dice-face-4 { transform: rotateY(-90deg) translateZ(50px); }
        .dice-face-5 { transform: rotateX(90deg) translateZ(50px); }
        .dice-face-6 { transform: rotateX(-90deg) translateZ(50px); }

        .dice-dots {
          display: grid;
          width: 80%;
          height: 80%;
          padding: 8px;
          gap: 4px;
        }

        .dice-face-1 .dice-dots { grid-template: 1fr / 1fr; place-items: center; }
        .dice-face-2 .dice-dots { grid-template: 1fr 1fr / 1fr; place-items: center; }
        .dice-face-3 .dice-dots { grid-template: 1fr 1fr 1fr / 1fr; place-items: center; }
        .dice-face-4 .dice-dots { grid-template: 1fr 1fr / 1fr 1fr; }
        .dice-face-5 .dice-dots { grid-template: 1fr 1fr 1fr / 1fr 1fr; }
        .dice-face-6 .dice-dots { grid-template: 1fr 1fr 1fr / 1fr 1fr; }

        .dice-dot {
          width: 14px;
          height: 14px;
          background: #333;
          border-radius: 50%;
        }

        .dice-face-2 .dice-dot:first-child { justify-self: start; align-self: start; }
        .dice-face-2 .dice-dot:last-child { justify-self: end; align-self: end; }

        .dice-face-3 .dice-dot:first-child { justify-self: start; align-self: start; }
        .dice-face-3 .dice-dot:nth-child(2) { justify-self: center; align-self: center; }
        .dice-face-3 .dice-dot:last-child { justify-self: end; align-self: end; }

        .dice-face-5 .dice-dot:nth-child(3) { grid-column: 1 / -1; justify-self: center; }
      `}</style>
    </div>
  )
}

export function DiceGame({ user, onClose, onResult }) {
  const [bet, setBet] = useState(10)
  const [target, setTarget] = useState(1)
  const [mode, setMode] = useState('exact') // 'exact', 'low', 'high'
  const [loading, setLoading] = useState(false)
  const [rolling, setRolling] = useState(false)
  const [result, setResult] = useState(null)
  const [displayValue, setDisplayValue] = useState(1)

  // Calculate win chance and multiplier based on mode
  const { winChance, multiplier } = useMemo(() => {
    if (mode === 'exact') {
      return { winChance: 16.67, multiplier: 5.5 }
    } else {
      return { winChance: 50, multiplier: 1.8 }
    }
  }, [mode])

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

      const response = await playDice(bet, target, mode)

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
        {/* 3D Dice display */}
        <div className="flex justify-center py-6">
          <div className={result ? (result.won ? 'dice-glow-success' : 'dice-glow-danger') : ''}>
            <Dice3D value={displayValue} rolling={rolling} />
          </div>
        </div>
        <style jsx>{`
          .dice-glow-success {
            filter: drop-shadow(0 0 20px rgba(34, 197, 94, 0.6));
            animation: pulse 1s infinite;
          }
          .dice-glow-danger {
            filter: drop-shadow(0 0 20px rgba(239, 68, 68, 0.6));
            animation: pulse 1s infinite;
          }
          @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.7; }
          }
        `}</style>

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
              {result.mode === 'exact' && `Your pick: ${target} → Rolled: ${result.result}`}
              {result.mode === 'low' && `Low (1-3) → Rolled: ${result.result}`}
              {result.mode === 'high' && `High (4-6) → Rolled: ${result.result}`}
            </div>
          </div>
        )}

        {result?.error && (
          <div className="text-center text-danger">{result.error}</div>
        )}

        {/* Game controls */}
        {!result && (
          <>
            {/* Game mode selection */}
            <div className="space-y-2">
              <label className="text-sm text-white/60">Select game mode</label>
              <div className="grid grid-cols-3 gap-2">
                <button
                  onClick={() => setMode('low')}
                  className={`py-3 px-2 rounded-xl font-medium transition-all ${
                    mode === 'low'
                      ? 'bg-primary text-white scale-105 shadow-lg'
                      : 'bg-white/10 text-white/60 hover:bg-white/20'
                  }`}
                >
                  <div className="text-lg font-bold">Low</div>
                  <div className="text-xs opacity-80">1-3</div>
                  <div className="text-xs text-success">{multiplier}x</div>
                </button>
                <button
                  onClick={() => setMode('high')}
                  className={`py-3 px-2 rounded-xl font-medium transition-all ${
                    mode === 'high'
                      ? 'bg-primary text-white scale-105 shadow-lg'
                      : 'bg-white/10 text-white/60 hover:bg-white/20'
                  }`}
                >
                  <div className="text-lg font-bold">High</div>
                  <div className="text-xs opacity-80">4-6</div>
                  <div className="text-xs text-success">{multiplier}x</div>
                </button>
                <button
                  onClick={() => setMode('exact')}
                  className={`py-3 px-2 rounded-xl font-medium transition-all ${
                    mode === 'exact'
                      ? 'bg-primary text-white scale-105 shadow-lg'
                      : 'bg-white/10 text-white/60 hover:bg-white/20'
                  }`}
                >
                  <div className="text-lg font-bold">Exact</div>
                  <div className="text-xs opacity-80">Pick #</div>
                  <div className="text-xs text-success">{multiplier}x</div>
                </button>
              </div>
            </div>

            {/* Pick your number (only for exact mode) */}
            {mode === 'exact' && (
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
              </div>
            )}

            {/* Range description */}
            {mode !== 'exact' && (
              <div className="bg-white/5 rounded-xl p-3 text-center">
                <div className="text-sm text-white/60">
                  {mode === 'low' ? 'Win if dice shows 1, 2, or 3' : 'Win if dice shows 4, 5, or 6'}
                </div>
              </div>
            )}

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
