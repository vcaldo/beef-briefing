import { useState, useEffect, useCallback } from 'react'

import { apiClient } from '../../api/client'
import { addPageAction, noticeError } from '../../newrelic'

import type { TabId, ChatUser } from '../../types'

// Common IANA timezones for quick selection
const COMMON_TIMEZONES = [
  { value: 'America/Sao_Paulo', label: 'Brasilia (BRT)' },
  { value: 'America/New_York', label: 'New York (EST/EDT)' },
  { value: 'America/Los_Angeles', label: 'Los Angeles (PST/PDT)' },
  { value: 'America/Chicago', label: 'Chicago (CST/CDT)' },
  { value: 'Europe/London', label: 'London (GMT/BST)' },
  { value: 'Europe/Paris', label: 'Paris (CET/CEST)' },
  { value: 'Europe/Berlin', label: 'Berlin (CET/CEST)' },
  { value: 'Asia/Tokyo', label: 'Tokyo (JST)' },
  { value: 'Asia/Shanghai', label: 'Shanghai (CST)' },
  { value: 'Asia/Dubai', label: 'Dubai (GST)' },
  { value: 'Australia/Sydney', label: 'Sydney (AEST/AEDT)' },
  { value: 'Pacific/Auckland', label: 'Auckland (NZST/NZDT)' },
  { value: 'UTC', label: 'UTC' },
]

interface AdminPageProps {
  chatTitle: string | null
  onTabChange?: (tab: TabId) => void
  onTimezoneChange?: (timezone: string | null) => void
  onImpersonate?: (userId: number | null) => void
  impersonatedUserId?: number | null
}

export function AdminPage({ chatTitle, onTabChange, onTimezoneChange, onImpersonate, impersonatedUserId }: AdminPageProps) {
  // Impersonation state
  const [users, setUsers] = useState<ChatUser[]>([])
  const [loadingUsers, setLoadingUsers] = useState(true)

  // Timezone state
  const [currentTimezone, setCurrentTimezone] = useState<string | null>(null)
  const [selectedTimezone, setSelectedTimezone] = useState<string>('')
  const [customTimezone, setCustomTimezone] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  const browserTimezone = apiClient.getBrowserTimezone()

  // Fetch users list for impersonation on mount
  useEffect(() => {
    async function fetchUsers() {
      try {
        const response = await apiClient.getChatUsers()
        setUsers(response.users)
      } catch (err) {
        console.error('Failed to fetch users:', err)
      } finally {
        setLoadingUsers(false)
      }
    }
    fetchUsers()
  }, [])

  // Fetch current timezone on mount
  useEffect(() => {
    async function fetchTimezone() {
      try {
        const response = await apiClient.getGroupTimezone()
        setCurrentTimezone(response.timezone)
        if (response.timezone) {
          // Check if it's in our common list
          if (COMMON_TIMEZONES.some(tz => tz.value === response.timezone)) {
            setSelectedTimezone(response.timezone)
          } else {
            setSelectedTimezone('custom')
            setCustomTimezone(response.timezone)
          }
        }
      } catch (err) {
        console.error('Failed to fetch timezone:', err)
        setError('Failed to load timezone settings')
        if (err instanceof Error) {
          noticeError(err, { context: 'admin_fetch_timezone' })
        }
      } finally {
        setLoading(false)
      }
    }
    fetchTimezone()
  }, [])

  const handleSave = useCallback(async () => {
    const timezoneToSave = selectedTimezone === 'custom' ? customTimezone : selectedTimezone

    if (!timezoneToSave) {
      setError('Please select a timezone')
      return
    }

    setSaving(true)
    setError(null)
    setSuccess(null)

    try {
      const response = await apiClient.setGroupTimezone(timezoneToSave)
      setCurrentTimezone(response.timezone)
      setSuccess('Timezone saved successfully!')
      addPageAction('admin_timezone_set', { timezone: timezoneToSave })
      onTimezoneChange?.(response.timezone)

      // Clear success message after 3 seconds
      setTimeout(() => setSuccess(null), 3000)
    } catch (err) {
      console.error('Failed to save timezone:', err)
      setError(err instanceof Error ? err.message : 'Failed to save timezone')
      if (err instanceof Error) {
        noticeError(err, { context: 'admin_set_timezone' })
      }
    } finally {
      setSaving(false)
    }
  }, [selectedTimezone, customTimezone, onTimezoneChange])

  const handleBack = useCallback(() => {
    onTabChange?.('profile')
  }, [onTabChange])

  return (
    <div className="page-container">
      <header className="app-header">
        <button className="admin-back-btn" onClick={handleBack}>
          <span className="admin-back-arrow">&larr;</span> Back
        </button>
        <h1>Admin Settings</h1>
        {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
      </header>

      {/* Impersonation Section */}
      <section className="admin-settings-section">
        <h2 className="section-title">Impersonate User</h2>
        <p className="section-subtitle">
          Select a user to view their profile and stats.
        </p>

        {loadingUsers ? (
          <div className="admin-loading">
            <div className="skeleton skeleton-row" style={{ height: '44px' }} />
          </div>
        ) : (
          <select
            className="admin-selector-dropdown"
            value={impersonatedUserId || ''}
            onChange={(e) => onImpersonate?.(e.target.value ? Number(e.target.value) : null)}
          >
            <option value="">-- My Profile --</option>
            {users.map((user) => (
              <option key={user.user_id} value={user.user_id}>
                {user.first_name} {user.last_name || ''} {user.username ? `(@${user.username})` : ''}
              </option>
            ))}
          </select>
        )}
      </section>

      {/* Timezone Section */}
      <section className="admin-settings-section">
        <h2 className="section-title">Group Timezone</h2>
        <p className="section-subtitle">
          Set the timezone for streak calculations and activity displays.
          This affects how daily streaks are counted for all users in the group.
        </p>

        {loading ? (
          <div className="admin-loading">
            <div className="skeleton skeleton-row" style={{ height: '60px' }} />
            <div className="skeleton skeleton-row" style={{ height: '44px', marginTop: '16px' }} />
          </div>
        ) : (
          <>
            <div className="admin-timezone-info">
              <div className="admin-info-row">
                <span className="admin-info-label">Current Setting</span>
                <span className="admin-info-value">
                  {currentTimezone || 'Not set (using UTC)'}
                </span>
              </div>
              <div className="admin-info-row">
                <span className="admin-info-label">Your Browser</span>
                <span className="admin-info-value">{browserTimezone}</span>
              </div>
            </div>

            <div className="admin-form-group">
              <label className="admin-form-label">Select Timezone</label>
              <select
                className="admin-selector-dropdown"
                value={selectedTimezone}
                onChange={(e) => {
                  setSelectedTimezone(e.target.value)
                  if (e.target.value !== 'custom') {
                    setCustomTimezone('')
                  }
                }}
                disabled={saving}
              >
                <option value="">-- Select a timezone --</option>
                {COMMON_TIMEZONES.map((tz) => (
                  <option key={tz.value} value={tz.value}>
                    {tz.label}
                  </option>
                ))}
                <option value="custom">Custom...</option>
              </select>
            </div>

            {selectedTimezone === 'custom' && (
              <div className="admin-form-group">
                <label className="admin-form-label">Custom Timezone</label>
                <input
                  type="text"
                  className="admin-input"
                  placeholder="e.g., America/Sao_Paulo"
                  value={customTimezone}
                  onChange={(e) => setCustomTimezone(e.target.value)}
                  disabled={saving}
                />
                <p className="admin-form-hint">
                  Enter a valid IANA timezone identifier
                </p>
              </div>
            )}

            {error && <div className="admin-message admin-message-error">{error}</div>}
            {success && <div className="admin-message admin-message-success">{success}</div>}

            <button
              className="admin-save-btn"
              onClick={handleSave}
              disabled={saving || (!selectedTimezone || (selectedTimezone === 'custom' && !customTimezone))}
            >
              {saving ? 'Saving...' : 'Save Timezone'}
            </button>
          </>
        )}
      </section>
    </div>
  )
}
