interface WeekSelectorProps {
  weeks: string[]
  selected: string | null
  onSelect: (week: string) => void
}

/**
 * Horizontal scrollable week selector with pill buttons.
 */
export function WeekSelector({ weeks, selected, onSelect }: WeekSelectorProps) {
  const formatWeek = (dateStr: string): string => {
    const date = new Date(dateStr)
    const month = date.toLocaleDateString('en-US', { month: 'short' })
    const day = date.getDate()
    return `${month} ${day}`
  }

  const getWeekLabel = (dateStr: string, index: number): string => {
    if (index === 0) {
      return `${formatWeek(dateStr)} (Latest)`
    }
    return formatWeek(dateStr)
  }

  return (
    <div className="week-selector">
      <div className="week-buttons">
        {weeks.map((week, index) => (
          <button
            key={week}
            className={`week-btn ${week === selected ? 'active' : ''}`}
            onClick={() => onSelect(week)}
          >
            {getWeekLabel(week, index)}
          </button>
        ))}
      </div>
    </div>
  )
}
