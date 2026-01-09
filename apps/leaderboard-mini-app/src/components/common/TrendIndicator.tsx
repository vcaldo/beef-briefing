interface TrendIndicatorProps {
  trend: number | null | undefined
}

export function TrendIndicator({ trend }: TrendIndicatorProps) {
  // Don't render if trend is null/undefined (no data available)
  if (trend === null || trend === undefined) {
    return null
  }

  const isPositive = trend > 0
  const isNegative = trend < 0
  const isNeutral = trend === 0

  // Cap display at +/-999%
  const displayValue = Math.min(Math.abs(trend), 999)
  const isCapped = Math.abs(trend) > 999

  const formattedTrend = `${isPositive ? '+' : isNegative ? '-' : ''}${displayValue.toFixed(0)}${isCapped ? '+' : ''}%`

  const className = [
    'trend-indicator',
    isPositive ? 'trend-up' : '',
    isNegative ? 'trend-down' : '',
    isNeutral ? 'trend-neutral' : '',
  ].filter(Boolean).join(' ')

  return (
    <span className={className}>
      {formattedTrend}
    </span>
  )
}
