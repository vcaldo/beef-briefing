/**
 * Beef Briefing Admin Panel - Charts JavaScript
 * ECharts initialization for calendar heatmap, timeline, and user activity charts
 */

// Wait for DOM to be ready
document.addEventListener('DOMContentLoaded', function() {
  initCalendar();
  initTimeline();
  initUserActivityChart();
});

// Re-initialize after HTMX swaps
document.body.addEventListener('htmx:afterSwap', function(event) {
  const targetId = event.detail.target.id;
  if (targetId === 'calendar-container' || targetId === 'heatmap-content') {
    initCalendar();
  }
  if (targetId === 'timeline-content') {
    initTimeline();
  }
  if (targetId === 'user-detail-content') {
    // Use requestAnimationFrame to ensure DOM is fully updated
    requestAnimationFrame(function() {
      initUserActivityChart();
    });
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
    fetch(`/chats/${chatIdAttr}/heatmap?year=${year}`)
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

/**
 * Initialize the timeline chart
 */
function initTimeline() {
  const timelineEl = document.getElementById('timeline-chart');
  if (!timelineEl) return;

  const dataAttr = timelineEl.getAttribute('data-timeline-data');
  const chatIdAttr = timelineEl.getAttribute('data-chat-id');
  const yearAttr = timelineEl.getAttribute('data-year');
  const monthAttr = timelineEl.getAttribute('data-month');
  const timezoneAttr = timelineEl.getAttribute('data-timezone');

  let data = [];

  // Parse data if provided via attribute
  if (dataAttr) {
    try {
      data = JSON.parse(dataAttr);
    } catch (e) {
      console.error('Failed to parse timeline data:', e);
    }
  }

  // If no data, fetch it
  if (data.length === 0 && chatIdAttr) {
    const year = yearAttr || new Date().getFullYear();
    const month = monthAttr || '0';
    const tz = timezoneAttr ? encodeURIComponent(timezoneAttr) : 'UTC';
    fetch(`/chats/${chatIdAttr}/timeline?year=${year}&month=${month}&tz=${tz}&granularity=week`)
      .then(response => response.text())
      .then(html => {
        const parser = new DOMParser();
        const doc = parser.parseFromString(html, 'text/html');
        const newTimelineEl = doc.getElementById('timeline-chart');
        if (newTimelineEl) {
          const newData = newTimelineEl.getAttribute('data-timeline-data');
          if (newData) {
            try {
              data = JSON.parse(newData);
              renderTimeline(timelineEl, data);
            } catch (e) {
              console.error('Failed to parse fetched timeline data:', e);
            }
          }
        }
      })
      .catch(err => console.error('Failed to fetch timeline data:', err));
    return;
  }

  renderTimeline(timelineEl, data);
}

/**
 * Render the timeline chart using ECharts
 */
function renderTimeline(element, data) {
  // Dispose existing chart if any
  const existingChart = echarts.getInstanceByDom(element);
  if (existingChart) {
    existingChart.dispose();
  }

  // Initialize new chart
  const chart = echarts.init(element, null, {
    renderer: 'canvas'
  });

  const isDark = document.documentElement.getAttribute('data-theme') !== 'light';

  // Extract data series
  const periods = data.map(d => d.period);
  const messages = data.map(d => d.messages);
  const users = data.map(d => d.users);
  const reactions = data.map(d => d.reactions);

  const option = {
    tooltip: {
      trigger: 'axis',
      backgroundColor: isDark ? 'rgba(15, 23, 42, 0.95)' : 'rgba(255, 255, 255, 0.95)',
      borderColor: isDark ? 'rgba(100, 116, 139, 0.3)' : 'rgba(148, 163, 184, 0.3)',
      textStyle: {
        color: isDark ? '#e2e8f0' : '#1e293b',
        fontFamily: 'JetBrains Mono, monospace',
        fontSize: 12
      },
      axisPointer: {
        type: 'shadow'
      }
    },
    legend: {
      data: ['Messages', 'Users', 'Reactions'],
      textStyle: {
        color: isDark ? '#94a3b8' : '#64748b',
        fontFamily: 'Space Grotesk, sans-serif'
      },
      top: 0
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '15%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: periods,
      axisLabel: {
        color: isDark ? '#64748b' : '#94a3b8',
        fontFamily: 'JetBrains Mono, monospace',
        fontSize: 10,
        rotate: 45
      },
      axisLine: {
        lineStyle: {
          color: isDark ? '#334155' : '#e2e8f0'
        }
      }
    },
    yAxis: [
      {
        type: 'value',
        name: 'Messages',
        nameTextStyle: {
          color: isDark ? '#94a3b8' : '#64748b',
          fontFamily: 'Space Grotesk, sans-serif'
        },
        axisLabel: {
          color: isDark ? '#64748b' : '#94a3b8',
          fontFamily: 'JetBrains Mono, monospace',
          fontSize: 10
        },
        splitLine: {
          lineStyle: {
            color: isDark ? '#1e293b' : '#f1f5f9'
          }
        }
      },
      {
        type: 'value',
        name: 'Users/Reactions',
        nameTextStyle: {
          color: isDark ? '#94a3b8' : '#64748b',
          fontFamily: 'Space Grotesk, sans-serif'
        },
        axisLabel: {
          color: isDark ? '#64748b' : '#94a3b8',
          fontFamily: 'JetBrains Mono, monospace',
          fontSize: 10
        },
        splitLine: {
          show: false
        }
      }
    ],
    series: [
      {
        name: 'Messages',
        type: 'bar',
        data: messages,
        itemStyle: {
          color: isDark ? '#3b82f6' : '#2563eb',
          borderRadius: [4, 4, 0, 0]
        }
      },
      {
        name: 'Users',
        type: 'line',
        yAxisIndex: 1,
        data: users,
        smooth: true,
        itemStyle: {
          color: isDark ? '#22d3ee' : '#06b6d4'
        },
        lineStyle: {
          width: 2
        }
      },
      {
        name: 'Reactions',
        type: 'line',
        yAxisIndex: 1,
        data: reactions,
        smooth: true,
        itemStyle: {
          color: isDark ? '#f472b6' : '#ec4899'
        },
        lineStyle: {
          width: 2
        }
      }
    ]
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
        renderTimeline(element, data);
      }
    });
  });

  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme']
  });
}

/**
 * Get timezone offset in hours from UTC for a given IANA timezone
 */
function getTimezoneOffset(timezone) {
  if (!timezone || timezone === 'UTC') return 0;

  try {
    // Create a date and format it in both UTC and the target timezone
    const now = new Date();
    const utcDate = new Date(now.toLocaleString('en-US', { timeZone: 'UTC' }));
    const tzDate = new Date(now.toLocaleString('en-US', { timeZone: timezone }));
    // Offset in hours (positive = ahead of UTC, negative = behind)
    return Math.round((tzDate - utcDate) / (1000 * 60 * 60));
  } catch (e) {
    console.error('Failed to get timezone offset:', e);
    return 0;
  }
}

/**
 * Shift an array of 24 hourly values by a timezone offset
 */
function shiftHourlyData(data, offsetHours) {
  if (offsetHours === 0 || data.length !== 24) return data;

  const shifted = new Array(24).fill(0);
  for (let i = 0; i < 24; i++) {
    // UTC hour i becomes local hour (i + offset) % 24
    let localHour = (i + offsetHours) % 24;
    if (localHour < 0) localHour += 24;
    shifted[localHour] = data[i];
  }
  return shifted;
}

/**
 * Initialize the user activity by hour chart
 */
function initUserActivityChart() {
  const activityEl = document.getElementById('user-activity-chart');
  if (!activityEl) return;

  const dataAttr = activityEl.getAttribute('data-activity-data');
  const timezone = activityEl.getAttribute('data-timezone') || 'UTC';
  let data = [];

  if (dataAttr) {
    try {
      data = JSON.parse(dataAttr);
    } catch (e) {
      console.error('Failed to parse activity data:', e);
      return;
    }
  }

  if (data.length === 0) return;

  // Shift data based on timezone offset
  const offset = getTimezoneOffset(timezone);
  const shiftedData = shiftHourlyData(data, offset);

  renderUserActivityChart(activityEl, shiftedData);
}

/**
 * Render the user activity by hour chart using ECharts
 */
function renderUserActivityChart(element, data) {
  // Dispose existing chart if any
  const existingChart = echarts.getInstanceByDom(element);
  if (existingChart) {
    existingChart.dispose();
  }

  // Initialize new chart
  const chart = echarts.init(element, null, {
    renderer: 'canvas'
  });

  const isDark = document.documentElement.getAttribute('data-theme') !== 'light';

  // Generate hour labels (0-23)
  const hours = Array.from({length: 24}, (_, i) => `${i}:00`);

  const option = {
    tooltip: {
      trigger: 'axis',
      backgroundColor: isDark ? 'rgba(15, 23, 42, 0.95)' : 'rgba(255, 255, 255, 0.95)',
      borderColor: isDark ? 'rgba(100, 116, 139, 0.3)' : 'rgba(148, 163, 184, 0.3)',
      textStyle: {
        color: isDark ? '#e2e8f0' : '#1e293b',
        fontFamily: 'JetBrains Mono, monospace',
        fontSize: 12
      },
      formatter: function(params) {
        return `${params[0].axisValue}<br/>Messages: ${params[0].data}`;
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '10%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: hours,
      axisLabel: {
        color: isDark ? '#64748b' : '#94a3b8',
        fontFamily: 'JetBrains Mono, monospace',
        fontSize: 9,
        interval: 3
      },
      axisLine: {
        lineStyle: {
          color: isDark ? '#334155' : '#e2e8f0'
        }
      }
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        color: isDark ? '#64748b' : '#94a3b8',
        fontFamily: 'JetBrains Mono, monospace',
        fontSize: 10
      },
      splitLine: {
        lineStyle: {
          color: isDark ? '#1e293b' : '#f1f5f9'
        }
      }
    },
    series: [{
      type: 'bar',
      data: data,
      itemStyle: {
        color: isDark ? '#8b5cf6' : '#7c3aed',
        borderRadius: [2, 2, 0, 0]
      }
    }]
  };

  chart.setOption(option);

  // Handle window resize
  window.addEventListener('resize', function() {
    chart.resize();
  });
}

// Export for use in HTMX callbacks
window.initCalendar = initCalendar;
window.initTimeline = initTimeline;
window.initUserActivityChart = initUserActivityChart;
