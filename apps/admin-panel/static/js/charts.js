/**
 * Beef Briefing Admin Panel - Charts JavaScript
 * ECharts initialization for calendar heatmap
 */

// Wait for DOM to be ready
document.addEventListener('DOMContentLoaded', function() {
  initCalendar();
});

// Re-initialize after HTMX swaps
document.body.addEventListener('htmx:afterSwap', function(event) {
  if (event.detail.target.id === 'calendar-container') {
    initCalendar();
  }
});

/**
 * Initialize the activity calendar heatmap
 */
function initCalendar() {
  const calendarEl = document.getElementById('activity-calendar');
  if (!calendarEl) return;

  // Get data from element attributes
  const dataAttr = calendarEl.getAttribute('data-calendar-data');
  const yearAttr = calendarEl.getAttribute('data-year');
  const chatIdAttr = calendarEl.getAttribute('data-chat-id');

  let data = [];
  let year = parseInt(yearAttr) || new Date().getFullYear();

  // Parse data if provided via attribute
  if (dataAttr) {
    try {
      data = JSON.parse(dataAttr);
    } catch (e) {
      console.error('Failed to parse calendar data:', e);
    }
  }

  // If no data attribute but we have chat ID, fetch data
  if (data.length === 0 && chatIdAttr) {
    fetch(`/chats/${chatIdAttr}/calendar-data?year=${year}`)
      .then(response => response.text())
      .then(html => {
        // The response is HTML with the calendar element, we need to extract data
        const parser = new DOMParser();
        const doc = parser.parseFromString(html, 'text/html');
        const newCalendarEl = doc.getElementById('activity-calendar');
        if (newCalendarEl) {
          const newData = newCalendarEl.getAttribute('data-calendar-data');
          if (newData) {
            try {
              data = JSON.parse(newData);
              renderCalendar(calendarEl, data, year);
            } catch (e) {
              console.error('Failed to parse fetched calendar data:', e);
            }
          }
        }
      })
      .catch(err => console.error('Failed to fetch calendar data:', err));
    return;
  }

  renderCalendar(calendarEl, data, year);
}

/**
 * Render the calendar heatmap using ECharts
 */
function renderCalendar(element, data, year) {
  // Dispose existing chart if any
  const existingChart = echarts.getInstanceByDom(element);
  if (existingChart) {
    existingChart.dispose();
  }

  // Initialize new chart
  const chart = echarts.init(element, null, {
    renderer: 'canvas'
  });

  // Calculate max value for color scaling
  const maxValue = data.length > 0
    ? Math.max(...data.map(d => d[1] || 0))
    : 10;

  // Get theme colors
  const style = getComputedStyle(document.documentElement);
  const isDark = document.documentElement.getAttribute('data-theme') !== 'light';

  const option = {
    tooltip: {
      position: 'top',
      formatter: function(params) {
        if (!params.data || !params.data[0]) return '';
        const date = params.data[0];
        const count = params.data[1] || 0;
        return `<strong>${date}</strong><br/>${count} message${count !== 1 ? 's' : ''}`;
      },
      backgroundColor: isDark ? 'rgba(15, 23, 42, 0.95)' : 'rgba(255, 255, 255, 0.95)',
      borderColor: isDark ? 'rgba(100, 116, 139, 0.3)' : 'rgba(148, 163, 184, 0.3)',
      textStyle: {
        color: isDark ? '#e2e8f0' : '#1e293b',
        fontFamily: 'JetBrains Mono, monospace',
        fontSize: 12
      }
    },
    visualMap: {
      show: false,
      min: 0,
      max: maxValue,
      inRange: {
        color: isDark
          ? ['#1e293b', '#0ea5e9', '#06b6d4', '#22d3ee']
          : ['#e2e8f0', '#3b82f6', '#2563eb', '#1d4ed8']
      }
    },
    calendar: {
      top: 30,
      left: 30,
      right: 30,
      bottom: 10,
      cellSize: ['auto', 15],
      range: year.toString(),
      itemStyle: {
        borderWidth: 2,
        borderColor: isDark ? '#1e293b' : '#f1f5f9'
      },
      yearLabel: {
        show: false
      },
      monthLabel: {
        color: isDark ? '#94a3b8' : '#64748b',
        fontFamily: 'Space Grotesk, sans-serif',
        fontWeight: 600,
        fontSize: 11
      },
      dayLabel: {
        color: isDark ? '#64748b' : '#94a3b8',
        fontFamily: 'JetBrains Mono, monospace',
        fontSize: 10,
        firstDay: 0,
        nameMap: ['S', 'M', 'T', 'W', 'T', 'F', 'S']
      },
      splitLine: {
        show: false
      }
    },
    series: [{
      type: 'heatmap',
      coordinateSystem: 'calendar',
      data: data
    }]
  };

  chart.setOption(option);

  // Handle window resize
  window.addEventListener('resize', function() {
    chart.resize();
  });

  // Re-render on theme change
  const observer = new MutationObserver(function(mutations) {
    mutations.forEach(function(mutation) {
      if (mutation.attributeName === 'data-theme') {
        renderCalendar(element, data, year);
      }
    });
  });

  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme']
  });
}

// Export for use in HTMX callbacks
window.initCalendar = initCalendar;
