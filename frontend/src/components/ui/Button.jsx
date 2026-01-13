export function Button({
  children,
  variant = 'primary',
  size = 'md',
  disabled = false,
  className = '',
  onClick,
  ...props
}) {
  const baseClasses = 'font-semibold rounded-xl transition-all duration-200 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100'

  const variants = {
    primary: 'bg-gradient-to-r from-primary to-secondary text-white btn-glow',
    secondary: 'bg-white/10 text-white hover:bg-white/20',
    success: 'bg-gradient-to-r from-emerald-500 to-green-500 text-white',
    danger: 'bg-gradient-to-r from-red-500 to-pink-500 text-white',
    ghost: 'bg-transparent text-white hover:bg-white/10',
  }

  const sizes = {
    sm: 'px-3 py-1.5 text-sm',
    md: 'px-4 py-2.5 text-base',
    lg: 'px-6 py-3 text-lg',
    xl: 'px-8 py-4 text-xl',
  }

  return (
    <button
      className={`${baseClasses} ${variants[variant]} ${sizes[size]} ${className}`}
      disabled={disabled}
      onClick={onClick}
      {...props}
    >
      {children}
    </button>
  )
}
