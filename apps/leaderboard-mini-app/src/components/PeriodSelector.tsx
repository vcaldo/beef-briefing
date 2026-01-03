import type { Period } from '../types'

interface PeriodSelectorProps {
  selectedPeriod: Period
  onPeriodChange: (period: Period) => void
}

const PERIODS: { value: Period; label: string }[] = [
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
  { value: '90d', label: '90d' },
  { value: '180d', label: '180d' },
  { value: '365d', label: '365d' },
  { value: 'ytd', label: 'YTD' },
  { value: 'max', label: 'MAX' },
]

export function PeriodSelector({ selectedPeriod, onPeriodChange }: PeriodSelectorProps) {
  return (
    <div className="period-selector">
      <div className="period-buttons">
        {PERIODS.map((period) => (
          <button
            key={period.value}
            className={`period-btn ${selectedPeriod === period.value ? 'active' : ''}`}
            onClick={() => onPeriodChange(period.value)}
          >
            {period.label}
          </button>
        ))}
      </div>
    </div>
  )
}
