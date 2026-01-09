import { useState, useEffect } from 'react'
import { useChat } from '../App'
import { api } from '../api/client'
import type { Message, MessageFilters, MessagesResponse } from '../types'

function SentimentBadge({ label }: { label: string | null }) {
  if (!label) return null
  const className = `badge badge-${label}`
  return <span className={className}>{label}</span>
}

function MessageCard({ message, onClick }: { message: Message; onClick: () => void }) {
  const userName = message.first_name
    ? `${message.first_name}${message.last_name ? ' ' + message.last_name : ''}`
    : message.username || 'Unknown'

  return (
    <div className="message-card" onClick={onClick}>
      <div className="message-header">
        <div className="message-user">{userName}</div>
        <div className="message-date">
          {new Date(message.date).toLocaleString()}
        </div>
      </div>
      <div className="message-text">
        {message.text.length > 300 ? message.text.slice(0, 300) + '...' : message.text}
      </div>
      <div className="message-badges">
        <SentimentBadge label={message.sentiment_label} />
        {message.is_toxic && <span className="badge badge-toxic">Toxic</span>}
        {message.is_humorous && <span className="badge badge-humor">Humor</span>}
        {message.is_question && <span className="badge badge-question">Question</span>}
        {message.topic_id !== null && message.topic_id >= 0 && (
          <span className="badge badge-entity">Topic {message.topic_id}</span>
        )}
      </div>
    </div>
  )
}

function MessageDetail({ messageId, onClose }: { messageId: number; onClose: () => void }) {
  const [data, setData] = useState<{ message: Message; entities: Array<{ entity_type: string; entity_text: string; confidence: number | null }> } | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.getMessage(messageId).then(setData).finally(() => setLoading(false))
  }, [messageId])

  if (loading) return <div className="loading">Loading...</div>
  if (!data) return null

  const { message, entities } = data
  const userName = message.first_name
    ? `${message.first_name}${message.last_name ? ' ' + message.last_name : ''}`
    : message.username || 'Unknown'

  return (
    <div className="card" style={{ marginBottom: 'var(--spacing-lg)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 'var(--spacing-md)' }}>
        <div>
          <div style={{ fontWeight: 600, fontSize: '16px' }}>{userName}</div>
          <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
            {new Date(message.date).toLocaleString()} | ID: {message.id}
          </div>
        </div>
        <button
          onClick={onClose}
          style={{
            background: 'none',
            border: 'none',
            color: 'var(--text-muted)',
            fontSize: '20px',
            cursor: 'pointer',
          }}
        >
          &times;
        </button>
      </div>

      <div style={{ marginBottom: 'var(--spacing-md)', lineHeight: 1.6 }}>
        {message.text}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 'var(--spacing-md)', marginBottom: 'var(--spacing-md)' }}>
        {message.sentiment_label && (
          <div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>SENTIMENT</div>
            <SentimentBadge label={message.sentiment_label} />
            <div style={{ fontFamily: 'var(--font-mono)', fontSize: '12px', marginTop: '4px', color: 'var(--text-secondary)' }}>
              +{(message.score_positive! * 100).toFixed(0)}% |
              ~{(message.score_neutral! * 100).toFixed(0)}% |
              -{(message.score_negative! * 100).toFixed(0)}%
            </div>
          </div>
        )}
        {message.is_toxic !== null && (
          <div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>TOXICITY</div>
            {message.is_toxic ? (
              <span className="badge badge-toxic">Toxic ({(message.toxicity_score! * 100).toFixed(0)}%)</span>
            ) : (
              <span style={{ color: 'var(--text-secondary)' }}>Not toxic</span>
            )}
          </div>
        )}
        {message.is_humorous !== null && (
          <div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>HUMOR</div>
            {message.is_humorous ? (
              <span className="badge badge-humor">{message.humor_type} ({(message.humor_score! * 100).toFixed(0)}%)</span>
            ) : (
              <span style={{ color: 'var(--text-secondary)' }}>Not humorous</span>
            )}
          </div>
        )}
        {message.is_question !== null && (
          <div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>QUESTION</div>
            {message.is_question ? (
              <span className="badge badge-question">{message.question_type}</span>
            ) : (
              <span style={{ color: 'var(--text-secondary)' }}>Not a question</span>
            )}
          </div>
        )}
      </div>

      {entities.length > 0 && (
        <div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '8px' }}>ENTITIES</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
            {entities.map((e, i) => (
              <span key={i} className="badge badge-entity">
                {e.entity_type}: {e.entity_text}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

export default function MessagesPage() {
  const { chatId } = useChat()
  const [messages, setMessages] = useState<Message[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [filters, setFilters] = useState<MessageFilters>({
    sort_by: 'date',
    sort_order: 'desc',
  })

  const limit = 50

  useEffect(() => {
    if (!chatId) return
    setLoading(true)
    api
      .getMessages(chatId, limit, offset, filters)
      .then((res) => {
        setMessages(res.messages)
        setTotal(res.total)
      })
      .finally(() => setLoading(false))
  }, [chatId, offset, filters])

  const handleFilterChange = (key: keyof MessageFilters, value: string | boolean | undefined) => {
    setOffset(0)
    setFilters((prev) => ({
      ...prev,
      [key]: value === '' ? undefined : value,
    }))
  }

  if (!chatId) {
    return <div className="loading">Select a chat to view messages</div>
  }

  return (
    <div>
      <header className="page-header">
        <h1 className="page-title">Messages</h1>
        <p className="page-subtitle">Browse messages with ML analysis results</p>
      </header>

      {/* Filters */}
      <div className="filters-bar">
        <div className="filter-group">
          <label className="filter-label">Sentiment</label>
          <select
            className="filter-select"
            value={filters.sentiment || ''}
            onChange={(e) => handleFilterChange('sentiment', e.target.value as 'positive' | 'neutral' | 'negative' | undefined)}
          >
            <option value="">All</option>
            <option value="positive">Positive</option>
            <option value="neutral">Neutral</option>
            <option value="negative">Negative</option>
          </select>
        </div>
        <div className="filter-group">
          <label className="filter-label">Toxic</label>
          <select
            className="filter-select"
            value={filters.is_toxic === undefined ? '' : filters.is_toxic.toString()}
            onChange={(e) => handleFilterChange('is_toxic', e.target.value === '' ? undefined : e.target.value === 'true')}
          >
            <option value="">All</option>
            <option value="true">Yes</option>
            <option value="false">No</option>
          </select>
        </div>
        <div className="filter-group">
          <label className="filter-label">Humorous</label>
          <select
            className="filter-select"
            value={filters.is_humorous === undefined ? '' : filters.is_humorous.toString()}
            onChange={(e) => handleFilterChange('is_humorous', e.target.value === '' ? undefined : e.target.value === 'true')}
          >
            <option value="">All</option>
            <option value="true">Yes</option>
            <option value="false">No</option>
          </select>
        </div>
        <div className="filter-group">
          <label className="filter-label">Question</label>
          <select
            className="filter-select"
            value={filters.is_question === undefined ? '' : filters.is_question.toString()}
            onChange={(e) => handleFilterChange('is_question', e.target.value === '' ? undefined : e.target.value === 'true')}
          >
            <option value="">All</option>
            <option value="true">Yes</option>
            <option value="false">No</option>
          </select>
        </div>
        <div className="filter-group">
          <label className="filter-label">Sort By</label>
          <select
            className="filter-select"
            value={filters.sort_by || 'date'}
            onChange={(e) => handleFilterChange('sort_by', e.target.value as 'date' | 'toxicity_score' | 'sentiment_score')}
          >
            <option value="date">Date</option>
            <option value="toxicity_score">Toxicity</option>
            <option value="sentiment_score">Sentiment</option>
          </select>
        </div>
        <div className="filter-group">
          <label className="filter-label">Order</label>
          <select
            className="filter-select"
            value={filters.sort_order || 'desc'}
            onChange={(e) => handleFilterChange('sort_order', e.target.value as 'asc' | 'desc')}
          >
            <option value="desc">Newest</option>
            <option value="asc">Oldest</option>
          </select>
        </div>
      </div>

      {/* Selected message detail */}
      {selectedId && (
        <MessageDetail messageId={selectedId} onClose={() => setSelectedId(null)} />
      )}

      {/* Messages list */}
      {loading ? (
        <div className="loading">Loading messages...</div>
      ) : (
        <>
          <div style={{ marginBottom: 'var(--spacing-md)', color: 'var(--text-secondary)', fontSize: '13px' }}>
            Showing {offset + 1}-{Math.min(offset + messages.length, total)} of {total.toLocaleString()} messages
          </div>
          {messages.map((msg) => (
            <MessageCard key={msg.id} message={msg} onClick={() => setSelectedId(msg.id)} />
          ))}
          <div className="pagination">
            <button
              className="pagination-btn"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - limit))}
            >
              Previous
            </button>
            <span className="pagination-info">
              Page {Math.floor(offset / limit) + 1} of {Math.ceil(total / limit)}
            </span>
            <button
              className="pagination-btn"
              disabled={offset + limit >= total}
              onClick={() => setOffset(offset + limit)}
            >
              Next
            </button>
          </div>
        </>
      )}
    </div>
  )
}
