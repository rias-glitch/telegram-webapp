export function Input({
  type = 'text',
  value,
  onChange,
  placeholder,
  disabled = false,
  className = '',
  ...props
}) {
  return (
    <input
      type={type}
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      disabled={disabled}
      className={`w-full bg-white/10 border border-white/20 rounded-xl px-4 py-3 text-white placeholder-white/50 focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary transition-colors disabled:opacity-50 ${className}`}
      {...props}
    />
  )
}

export function BetInput({ value, onChange, min = 1, max, disabled = false }) {
  const presets = [10, 50, 100, 500]

  const handlePreset = (amount) => {
    if (!disabled) {
      onChange(amount)
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <Input
          type="number"
          value={value}
          onChange={(e) => onChange(parseInt(e.target.value) || 0)}
          min={min}
          max={max}
          disabled={disabled}
          className="text-center text-xl font-bold"
        />
        <span className="text-2xl">💎</span>
      </div>
      <div className="flex gap-2">
        {presets.map((amount) => (
          <button
            key={amount}
            onClick={() => handlePreset(amount)}
            disabled={disabled || (max && amount > max)}
            className="flex-1 py-2 bg-white/10 rounded-lg text-sm font-medium hover:bg-white/20 transition-colors disabled:opacity-50"
          >
            {amount}
          </button>
        ))}
      </div>
    </div>
  )
}
