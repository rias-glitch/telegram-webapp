import { useState, useEffect, useRef } from 'react'
import { Modal } from '../ui/Overlay'
import { Button, Input } from '../ui'
import { playWheel, getWheelInfo } from '../../api/games'

const BET_PRESETS = [10, 50, 100, 500]

// Default segments matching backend
const DEFAULT_SEGMENTS = [
  { id: 1, multiplier: 0.0, color: '#4a4a4a', label: '0x' },
  { id: 2, multiplier: 0.5, color: '#e74c3c', label: '0.5x' },
  { id: 3, multiplier: 1.0, color: '#f39c12', label: '1x' },
  { id: 4, multiplier: 1.5, color: '#2ecc71', label: '1.5x' },
  { id: 5, multiplier: 2.0, color: '#3498db', label: '2x' },
  { id: 6, multiplier: 3.0, color: '#9b59b6', label: '3x' },
  { id: 7, multiplier: 5.0, color: '#e67e22', label: '5x' },
  { id: 8, multiplier: 10.0, color: '#f1c40f', label: '10x' },
]

export function WheelGame({ user, onClose, onResult }) {
  const [bet, setBet] = useState(10)
  const [loading, setLoading] = useState(false)
  const [spinning, setSpinning] = useState(false)
  const [result, setResult] = useState(null)
  const [segments, setSegments] = useState(DEFAULT_SEGMENTS)
  const [offset, setOffset] = useState(0)
  const stripRef = useRef(null)

  useEffect(() => {
    getWheelInfo().then(data => {
      if (data.segments) setSegments(data.segments)
    }).catch(() => {})
  }, [])

  // Create extended strip for smooth looping (repeat segments multiple times)
  const extendedSegments = [...segments, ...segments, ...segments, ...segments, ...segments]
  const segmentWidth = 80 // px per segment

  const handleSpin = async () => {
    if (bet <= 0 || bet > (user?.gems || 0)) return

    try {
      setLoading(true)
      setSpinning(true)
      setResult(null)

      const response = await playWheel(bet)

      // Calculate target position
      // Find winning segment index and add random rotations
      const winIndex = segments.findIndex(s => s.id === response.segment_id)
      const rotations = 3 // Full rotations before stopping
      const totalSegments = segments.length
      const targetPosition = (rotations * totalSegments + winIndex) * segmentWidth

      // Add some randomness within the segment
      const randomOffset = Math.random() * (segmentWidth * 0.6) - (segmentWidth * 0.3)

      setOffset(targetPosition + randomOffset)

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
    setOffset(0)
  }

  return (
    <Modal isOpen={true} onClose={onClose} title="Wheel of Fortune">
      <div className="space-y-6">
        {/* Slot machine style display */}
        <div className="relative">
          {/* Frame */}
          <div className="bg-gradient-to-b from-yellow-600 to-yellow-800 rounded-2xl p-2 shadow-lg">
            <div className="bg-dark rounded-xl p-1 relative overflow-hidden">
              {/* Pointer/Indicator */}
              <div className="absolute left-1/2 top-0 -translate-x-1/2 z-20 w-0 h-0 border-l-[12px] border-r-[12px] border-t-[16px] border-l-transparent border-r-transparent border-t-yellow-400" />
              <div className="absolute left-1/2 bottom-0 -translate-x-1/2 z-20 w-0 h-0 border-l-[12px] border-r-[12px] border-b-[16px] border-l-transparent border-r-transparent border-b-yellow-400" />

              {/* Gradient overlays for depth effect */}
              <div className="absolute left-0 top-0 bottom-0 w-16 bg-gradient-to-r from-dark to-transparent z-10 pointer-events-none" />
              <div className="absolute right-0 top-0 bottom-0 w-16 bg-gradient-to-l from-dark to-transparent z-10 pointer-events-none" />

              {/* Strip container */}
              <div className="h-24 overflow-hidden relative">
                <div
                  ref={stripRef}
                  className="flex absolute"
                  style={{
                    transform: `translateX(calc(50% - ${offset}px - ${segmentWidth / 2}px))`,
                    transition: spinning ? 'transform 3s cubic-bezier(0.15, 0.85, 0.3, 1)' : 'none',
                  }}
                >
                  {extendedSegments.map((seg, i) => (
                    <div
                      key={i}
                      className="flex-shrink-0 h-24 flex items-center justify-center font-bold text-xl border-x border-white/20"
                      style={{
                        width: segmentWidth,
                        backgroundColor: seg.color,
                        textShadow: '0 2px 4px rgba(0,0,0,0.5)',
                      }}
                    >
                      {seg.label}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Decorative lights */}
          <div className="flex justify-around mt-2">
            {[...Array(7)].map((_, i) => (
              <div
                key={i}
                className={`w-3 h-3 rounded-full ${spinning ? 'animate-pulse' : ''}`}
                style={{
                  backgroundColor: spinning ? ['#ef4444', '#f59e0b', '#10b981'][i % 3] : '#666',
                  boxShadow: spinning ? `0 0 8px ${['#ef4444', '#f59e0b', '#10b981'][i % 3]}` : 'none',
                }}
              />
            ))}
          </div>
        </div>

        {/* Result */}
        {result && !result.error && (
          <div className="text-center space-y-2">
            <div
              className="text-4xl font-bold"
              style={{ color: result.color }}
            >
              {result.label}
            </div>
            <div className={`text-2xl font-bold ${result.multiplier >= 1 ? 'text-success' : 'text-danger'}`}>
              {result.win_amount > 0 ? '+' : ''}{result.win_amount - bet} gems
            </div>
          </div>
        )}

        {result?.error && (
          <div className="text-center text-danger">{result.error}</div>
        )}

        {/* Multipliers preview */}
        {!spinning && !result && (
          <div className="grid grid-cols-4 gap-2">
            {segments.map((seg) => (
              <div
                key={seg.id}
                className="text-center py-2 px-1 rounded-lg text-sm font-bold"
                style={{
                  backgroundColor: seg.color + '30',
                  color: seg.color,
                  border: `1px solid ${seg.color}50`,
                }}
              >
                {seg.label}
              </div>
            ))}
          </div>
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
                className="flex-1 bg-gradient-to-r from-yellow-500 to-orange-500 hover:from-yellow-600 hover:to-orange-600"
              >
                {spinning ? '🎰 Spinning...' : `🎰 Spin (${bet})`}
              </Button>
            </>
          )}
        </div>
      </div>
    </Modal>
  )
}
