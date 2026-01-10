import { useState, useEffect } from 'react'
import { useChat } from '../App'
import { api } from '../api/client'
import type { StatsResponse } from '../types'

export default function DashboardPage() {
  const { chatId, currentChat } = useChat()
  const [stats, setStats] = useState<StatsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    async function loadStats() {
      if (!chatId) return
      setLoading(true)
      setError(null)
      try {
        const data = await api.getStats(chatId)
        setStats(data)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load stats')
      } finally {
        setLoading(false)
      }
    }
    loadStats()
  }, [chatId])

  if (!chatId) {
    return <div className="loading">Select a chat to view stats</div>
  }

  if (loading) {
    return <div className="loading">Loading stats...</div>
  }

  if (error) {
    return <div className="error">{error}</div>
  }

  if (!stats) {
    return <div className="loading">No stats available</div>
  }

  const { processing, qdrant } = stats
  const processedPct = processing.total_with_text > 0
    ? Math.round((processing.processed / processing.total_with_text) * 100)
    : 0

  return (
    <div>
      <header className="page-header">
        <h1 className="page-title">Dashboard</h1>
        <p className="page-subtitle">
          {currentChat?.title || `Chat ${chatId}`} - ML Processing Overview
        </p>
      </header>

      {/* Processing Status */}
      <section style={{ marginBottom: 'var(--spacing-xl)' }}>
        <h2 style={{ fontSize: '16px', fontWeight: 600, marginBottom: 'var(--spacing-md)', color: 'var(--text-secondary)' }}>
          Processing Status
        </h2>
        <div className="card" style={{ marginBottom: 'var(--spacing-md)' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--spacing-sm)' }}>
            <span style={{ fontSize: '14px', color: 'var(--text-secondary)' }}>
              {processing.processed.toLocaleString()} / {processing.total_with_text.toLocaleString()} messages processed
            </span>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: '14px', color: 'var(--accent-green)' }}>
              {processedPct}%
            </span>
          </div>
          <div className="progress-bar">
            <div className="progress-fill" style={{ width: `${processedPct}%` }} />
          </div>
        </div>

        <div className="stats-grid">
          <div className="card">
            <div className="card-title">Total Messages</div>
            <div className="card-value">{processing.total_with_text.toLocaleString()}</div>
            <div className="card-label">with text content</div>
          </div>
          <div className="card">
            <div className="card-title">Processed</div>
            <div className="card-value">{processing.processed.toLocaleString()}</div>
            <div className="card-label">ML analyzed</div>
          </div>
        </div>
      </section>

      {/* ML Results */}
      <section style={{ marginBottom: 'var(--spacing-xl)' }}>
        <h2 style={{ fontSize: '16px', fontWeight: 600, marginBottom: 'var(--spacing-md)', color: 'var(--text-secondary)' }}>
          ML Analysis Results
        </h2>
        <div className="stats-grid">
          <div className="card">
            <div className="card-title">Sentiment Analysis</div>
            <div className="card-value">{processing.sentiment_count.toLocaleString()}</div>
            <div className="card-label">messages analyzed</div>
          </div>
          <div className="card">
            <div className="card-title">Toxicity Detection</div>
            <div className="card-value" style={{ color: 'var(--accent-red)' }}>
              {processing.toxic_count.toLocaleString()}
            </div>
            <div className="card-label">
              toxic messages ({processing.toxicity_count > 0
                ? ((processing.toxic_count / processing.toxicity_count) * 100).toFixed(1)
                : 0}%)
            </div>
          </div>
          <div className="card">
            <div className="card-title">Humor Detection</div>
            <div className="card-value" style={{ color: 'var(--accent-purple)' }}>
              {processing.humorous_count.toLocaleString()}
            </div>
            <div className="card-label">
              humorous messages ({processing.humor_count > 0
                ? ((processing.humorous_count / processing.humor_count) * 100).toFixed(1)
                : 0}%)
            </div>
          </div>
          <div className="card">
            <div className="card-title">Question Detection</div>
            <div className="card-value" style={{ color: 'var(--accent-blue)' }}>
              {processing.question_count.toLocaleString()}
            </div>
            <div className="card-label">
              questions ({processing.questions_count > 0
                ? ((processing.question_count / processing.questions_count) * 100).toFixed(1)
                : 0}%)
            </div>
          </div>
          <div className="card">
            <div className="card-title">Named Entities</div>
            <div className="card-value" style={{ color: 'var(--accent-cyan)' }}>
              {processing.entity_count.toLocaleString()}
            </div>
            <div className="card-label">entities extracted</div>
          </div>
          <div className="card">
            <div className="card-title">Topic Clusters</div>
            <div className="card-value">{processing.topic_count.toLocaleString()}</div>
            <div className="card-label">distinct topics</div>
          </div>
        </div>
      </section>

      {/* Qdrant Status */}
      <section>
        <h2 style={{ fontSize: '16px', fontWeight: 600, marginBottom: 'var(--spacing-md)', color: 'var(--text-secondary)' }}>
          Vector Database (Qdrant)
        </h2>
        <div className="card">
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--spacing-md)' }}>
            <div
              style={{
                width: 12,
                height: 12,
                borderRadius: '50%',
                backgroundColor: qdrant.available ? 'var(--accent-green)' : 'var(--accent-red)',
              }}
            />
            <div>
              <div style={{ fontWeight: 500 }}>
                {qdrant.available ? 'Connected' : 'Not Available'}
              </div>
              <div style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
                {qdrant.available
                  ? `${qdrant.points_count.toLocaleString()} embeddings stored`
                  : 'Semantic search disabled'}
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  )
}
