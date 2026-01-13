import { useState, useEffect } from 'react'
import { Modal } from '../ui/Overlay'
import { Button, Input } from '../ui'
import { playWheel, getWheelInfo } from '../../api/games'

const BET_PRESETS = [10, 50, 100, 500]

export function WheelGame({ user, onClose, onResult }) {
  const [bet, setBet] = useState(10)
  const [loading, setLoading] = useState(false)
  const [spinning, setSpinning] = useState(false)
  const [result, setResult] = useState(null)
  const [segments, setSegments] = useState([])
  const [rotation, setRotation] = useState(0)

  useEffect(() => {
    getWheelInfo().then(data => {
      if (data.segments) setSegments(data.segments)
    }).catch(() => {})
  }, [])

  const handleSpin = async () => {
    if (bet <= 0 || bet > (user?.gems || 0)) return

    try {
      setLoading(true)
      setSpinning(true)
      setResult(null)

      const response = await playWheel(bet)

      // Animate wheel
      setRotation(prev => prev + response.spin_angle)

      // Show result after animation
      setTimeout(() => {
        setSpinning(false)
        setResult(response)
        onResult(response.gems)
      }, 3000)
    } catch (err) {
      setSpinning(false)
      setResult({ error: err.message })
    } finally {
      setLoading(false)
    }
  }

  const playAgain = () => {
    setResult(null)
  }

  const segmentAngle = 360 / (segments.length || 8)

  return (
    <Modal isOpen={true} onClose={onClose} title="Wheel of Fortune">
      <div className="space-y-6">
        {/* Wheel */}
        <div className="relative flex justify-center py-4">
          {/* Pointer */}
          <div className="absolute top-2 left-1/2 -translate-x-1/2 z-10 text-3xl">
            ▼
          </div>

          {/* Wheel container */}
          <div
            className="w-64 h-64 rounded-full relative overflow-hidden border-4 border-white/20"
            style={{
              transform: `rotate(${rotation}deg)`,
              transition: spinning ? 'transform 3s cubic-bezier(0.17, 0.67, 0.12, 0.99)' : 'none',
            }}
          >
            {segments.map((seg, i) => (
              <div
                key={seg.id}
                className="absolute w-full h-full"
                style={{
                  transform: `rotate(${i * segmentAngle}deg)`,
                  clipPath: `polygon(50% 50%, 50% 0%, ${50 + 50 * Math.tan((segmentAngle * Math.PI) / 360)}% 0%)`,
                }}
              >
                <div
                  className="absolute inset-0 flex items-start justify-center pt-4"
                  style={{ backgroundColor: seg.color }}
                >
                  <span
                    className="text-xs font-bold text-white drop-shadow-lg"
                    style={{ transform: `rotate(${segmentAngle / 2}deg)` }}
                  >
                    {seg.label}
                  </span>
                </div>
              </div>
            ))}
            {/* Simple colored wheel fallback */}
            {segments.length === 0 && (
              <div className="w-full h-full bg-gradient-conic from-red-500 via-yellow-500 via-green-500 via-blue-500 to-purple-500 animate-pulse" />
            )}
          </div>
        </div>

        {/* Result */}
        {result && !result.error && (
          <div className="text-center space-y-2">
            <div
              className="text-3xl font-bold"
              style={{ color: result.color }}
            >
              {result.label}
            </div>
            <div className={`text-xl font-bold ${result.multiplier >= 1 ? 'text-success' : 'text-danger'}`}>
              {result.multiplier >= 1 ? '+' : ''}{result.win_amount - bet} gems
            </div>
          </div>
        )}

        {result?.error && (
          <div className="text-center text-danger">{result.error}</div>
        )}

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

        {/* Multipliers info */}
        {!spinning && !result && segments.length > 0 && (
          <div className="grid grid-cols-4 gap-2 text-xs">
            {segments.map((seg) => (
              <div
                key={seg.id}
                className="text-center py-1 px-2 rounded"
                style={{ backgroundColor: seg.color + '40' }}
              >
                {seg.label}
              </div>
            ))}
          </div>
        )}

        {/* Actions */}
        <div className="flex gap-3">
          {result ? (
            <>
              <Button variant="secondary" onClick={onClose} className="flex-1">
                Close
              </Button>
              <Button onClick={playAgain} className="flex-1">
                Spin Again
              </Button>
            </>
          ) : (
            <>
              <Button variant="secondary" onClick={onClose} className="flex-1">
                Cancel
              </Button>
              <Button
                onClick={handleSpin}
                disabled={loading || spinning || bet <= 0 || bet > (user?.gems || 0)}
                className="flex-1"
              >
                {spinning ? 'Spinning...' : `Spin (${bet})`}
              </Button>
            </>
          )}
        </div>
      </div>
    </Modal>
  )
}
