import type { HeatmapData } from '../../types'

const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const HOURS = Array.from({ length: 24 }, (_, i) => i)

interface HeatmapGridProps {
  data: HeatmapData
  title?: string
  showLabels?: boolean
}

export function HeatmapGrid({ data, title, showLabels = true }: HeatmapGridProps) {
  // Create a lookup map for quick cell access
  const cellMap = new Map<string, number>()
  // Defensive check for malformed data
  if (data?.data && Array.isArray(data.data)) {
    data.data.forEach((cell) => {
      cellMap.set(`${cell.day_of_week}-${cell.hour}`, cell.message_count)
    })
  }

  // Calculate color intensity based on count relative to max
  const maxCount = data?.max_count ?? 0
  const getColor = (count: number): string => {
    if (count === 0 || maxCount === 0) {
      return 'var(--tg-theme-secondary-bg-color)'
    }
    const intensity = count / maxCount
    // Use CSS variable for button color with varying opacity
    if (intensity > 0.75) return 'var(--heatmap-high)'
    if (intensity > 0.5) return 'var(--heatmap-medium-high)'
    if (intensity > 0.25) return 'var(--heatmap-medium)'
    return 'var(--heatmap-low)'
  }

  return (
    <div className="heatmap-container">
      {title && <h3 className="heatmap-title">{title}</h3>}
      <div className="heatmap-grid">
        {/* Empty corner cell */}
        {showLabels && <div className="heatmap-label corner"></div>}

        {/* Hour labels */}
        {showLabels && HOURS.map((hour) => (
          <div key={`h-${hour}`} className="heatmap-label hour">
            {hour % 6 === 0 ? `${hour}` : ''}
          </div>
        ))}

        {/* Day rows */}
        {DAYS.map((day, dayIndex) => (
          <>
            {/* Day label */}
            {showLabels && (
              <div key={`d-${dayIndex}`} className="heatmap-label day">
                {day}
              </div>
            )}

            {/* Hour cells for this day */}
            {HOURS.map((hour) => {
              const count = cellMap.get(`${dayIndex}-${hour}`) || 0
              return (
                <div
                  key={`${dayIndex}-${hour}`}
                  className="heatmap-cell"
                  style={{ backgroundColor: getColor(count) }}
                  title={`${day} ${hour}:00 - ${count} messages`}
                />
              )
            })}
          </>
        ))}
      </div>

      {/* Legend */}
      <div className="heatmap-legend">
        <span className="heatmap-legend-label">Less</span>
        <div className="heatmap-legend-scale">
          <div className="heatmap-cell" style={{ backgroundColor: 'var(--tg-theme-secondary-bg-color)' }} />
          <div className="heatmap-cell" style={{ backgroundColor: 'var(--heatmap-low)' }} />
          <div className="heatmap-cell" style={{ backgroundColor: 'var(--heatmap-medium)' }} />
          <div className="heatmap-cell" style={{ backgroundColor: 'var(--heatmap-medium-high)' }} />
          <div className="heatmap-cell" style={{ backgroundColor: 'var(--heatmap-high)' }} />
        </div>
        <span className="heatmap-legend-label">More</span>
      </div>

      <div className="heatmap-stats">
        Total: {(data?.total_messages ?? 0).toLocaleString()} messages
      </div>
    </div>
  )
}

// Skeleton loader for heatmap
export function HeatmapSkeleton() {
  return (
    <div className="heatmap-container">
      <div className="heatmap-grid skeleton-grid">
        <div className="heatmap-label corner"></div>
        {HOURS.map((hour) => (
          <div key={`h-${hour}`} className="heatmap-label hour skeleton"></div>
        ))}
        {DAYS.map((_, dayIndex) => (
          <>
            <div key={`d-${dayIndex}`} className="heatmap-label day skeleton"></div>
            {HOURS.map((hour) => (
              <div key={`${dayIndex}-${hour}`} className="heatmap-cell skeleton" />
            ))}
          </>
        ))}
      </div>
    </div>
  )
}
