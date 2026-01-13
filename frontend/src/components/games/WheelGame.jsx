import { useState, useEffect } from 'react'
import { Modal } from '../ui/Overlay'
import { Button, Input } from '../ui'
import { playWheel, getWheelInfo } from '../../api/games'

const BET_PRESETS = [10, 50, 100, 500]

// Default segments matching backend order
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
  const [displayIndex, setDisplayIndex] = useState(0)

  useEffect(() => {
    getWheelInfo().then(data => {
      if (data.segments && data.segments.length > 0) {
        setSegments(data.segments)
      }
    }).catch(() => {})
  }, [])

  const handleSpin = async () => {
    if (bet <= 0 || bet > (user?.gems || 0) || spinning) return

    try {
      setLoading(true)
      setSpinning(true)
      setResult(null)

      const response = await playWheel(bet)

      // Find winning segment index
      const winIndex = segments.findIndex(s => s.id === response.segment_id)
      const finalIndex = winIndex >= 0 ? winIndex : 0

      // Animate through segments
      const totalSpins = 3 * segments.length + finalIndex // 3 full rotations + final position
      let currentSpin = 0

      const spinInterval = setInterval(() => {
        currentSpin++
        setDisplayIndex(currentSpin % segments.length)

        if (currentSpin >= totalSpins) {
          clearInterval(spinInterval)
          setDisplayIndex(finalIndex)
          setSpinning(false)
          setResult(response)
          onResult(response.gems)
        }
      }, 100 - Math.min(currentSpin * 2, 70)) // Speed up then slow down

      // Fallback timeout
      setTimeout(() => {
        clearInterval(spinInterval)
        setDisplayIndex(finalIndex)
        setSpinning(false)
        setResult(response)
        onResult(response.gems)
      }, 4000)

    } catch (err) {
      setSpinning(false)
      setResult({ error: err.message })
    } finally {
      setLoading(false)
    }
  }

  const playAgain = () => {
    setResult(null)
    setDisplayIndex(0)
  }

  const currentSegment = segments[displayIndex] || segments[0]

  return (
    <Modal isOpen={true} onClose={onClose} title="Wheel of Fortune">
      <div className="space-y-6">
        {/* Slot machine display */}
        <div className="relative">
          {/* Frame */}
          <div className="bg-gradient-to-b from-yellow-600 to-yellow-800 rounded-2xl p-3 shadow-lg">
            {/* Main display */}
            <div
              className="h-32 rounded-xl flex items-center justify-center text-5xl font-bold transition-all duration-100"
              style={{
                backgroundColor: currentSegment.color,
                textShadow: '0 2px 8px rgba(0,0,0,0.5)',
              }}
            >
              {currentSegment.label}
            </div>
          </div>

          {/* Decorative lights */}
          <div className="flex justify-around mt-3">
            {[...Array(8)].map((_, i) => (
              <div
                key={i}
                className={`w-3 h-3 rounded-full transition-all`}
                style={{
                  backgroundColor: spinning
                    ? ['#ef4444', '#f59e0b', '#10b981', '#3b82f6'][i % 4]
                    : '#444',
                  boxShadow: spinning
                    ? `0 0 10px ${['#ef4444', '#f59e0b', '#10b981', '#3b82f6'][i % 4]}`
                    : 'none',
                  animation: spinning ? `pulse ${0.3 + i * 0.1}s infinite` : 'none',
                }}
              />
            ))}
          </div>
        </div>

        {/* Result */}
        {result && !result.error && (
          <div className="text-center space-y-2 animate-fadeIn">
            <div
              className="text-4xl font-bold"
              style={{ color: segments.find(s => s.id === result.segment_id)?.color || '#fff' }}
            >
              {result.label}
            </div>
            <div className={`text-2xl font-bold ${result.multiplier >= 1 ? 'text-success' : 'text-danger'}`}>
              {result.win_amount > bet ? '+' : ''}{result.win_amount - bet} gems
            </div>
            <div className="text-white/60">
              {result.multiplier}x multiplier
            </div>
          </div>
        )}

        {result?.error && (
          <div className="text-center text-danger">{result.error}</div>
        )}

        {/* Multipliers grid */}
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
