import { useState, useEffect } from 'react'
import { useChat } from '../App'
import { api } from '../api/client'
import type { SearchResult, SearchStatus } from '../types'

export default function SearchPage() {
  const { chatId } = useChat()
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [status, setStatus] = useState<SearchStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [timing, setTiming] = useState<{ embedding_ms: number; search_ms: number } | null>(null)
  const [error, setError] = useState<string | null>(null)

  // Load search status on mount
  useEffect(() => {
    api.getSearchStatus().then(setStatus).catch(() => {
      setStatus({ qdrant: { available: false, points_count: 0, status: 'unavailable' }, embedding_model_loaded: false, search_available: false })
    })
  }, [])

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!query.trim() || !chatId) return

    setLoading(true)
    setError(null)
    try {
      const response = await api.search(query, chatId, undefined, 20)
      setResults(response.results)
      setTiming(response.timing)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed')
      setResults([])
    } finally {
      setLoading(false)
    }
  }

  if (!chatId) {
    return <div className="loading">Select a chat to search</div>
  }

  const searchAvailable = status?.search_available ?? false

  return (
    <div>
      <header className="page-header">
        <h1 className="page-title">Semantic Search</h1>
        <p className="page-subtitle">Find similar messages using vector embeddings</p>
      </header>

      {/* Status indicator */}
      {status && (
        <div className="card" style={{ marginBottom: 'var(--spacing-lg)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--spacing-md)' }}>
            <div
              style={{
                width: 10,
                height: 10,
                borderRadius: '50%',
                backgroundColor: searchAvailable ? 'var(--accent-green)' : 'var(--accent-red)',
              }}
            />
            <div>
              {searchAvailable ? (
                <>
                  <span style={{ fontWeight: 500 }}>Qdrant Connected</span>
                  <span style={{ color: 'var(--text-muted)', marginLeft: 8 }}>
                    {status.qdrant.points_count.toLocaleString()} embeddings
                  </span>
                </>
              ) : (
                <>
                  <span style={{ fontWeight: 500, color: 'var(--accent-orange)' }}>
                    Semantic search unavailable
                  </span>
                  <span style={{ color: 'var(--text-muted)', marginLeft: 8 }}>
                    Qdrant is only available in dev environment
                  </span>
                </>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Search form */}
      <form onSubmit={handleSearch} className="search-container">
        <span className="search-icon">&#x1F50D;</span>
        <input
          type="text"
          className="search-input"
          placeholder={searchAvailable ? 'Search for similar messages...' : 'Search unavailable'}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          disabled={!searchAvailable || loading}
        />
      </form>

      {error && <div className="error" style={{ marginBottom: 'var(--spacing-lg)' }}>{error}</div>}

      {/* Results */}
      {loading ? (
        <div className="loading">Searching...</div>
      ) : results.length > 0 ? (
        <>
          <div style={{ marginBottom: 'var(--spacing-md)', color: 'var(--text-secondary)', fontSize: '13px' }}>
            Found {results.length} similar messages
            {timing && (
              <span style={{ color: 'var(--text-muted)', marginLeft: 8 }}>
                (embedding: {timing.embedding_ms.toFixed(0)}ms, search: {timing.search_ms.toFixed(0)}ms)
              </span>
            )}
          </div>
          {results.map((result, i) => (
            <div key={result.message_id} className="message-card" style={{ cursor: 'default' }}>
              <div className="message-header">
                <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--spacing-sm)' }}>
                  <span
                    style={{
                      fontFamily: 'var(--font-mono)',
                      fontSize: '12px',
                      color: 'var(--accent-purple)',
                      backgroundColor: 'rgba(163, 113, 247, 0.1)',
                      padding: '2px 6px',
                      borderRadius: '4px',
                    }}
                  >
                    #{i + 1}
                  </span>
                  <span className="message-user">
                    {result.message?.first_name || 'Unknown'}
                  </span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--spacing-sm)' }}>
                  <span
                    style={{
                      fontFamily: 'var(--font-mono)',
                      fontSize: '12px',
                      color: 'var(--accent-green)',
                    }}
                  >
                    {(result.score * 100).toFixed(1)}% match
                  </span>
                  {result.message && (
                    <span className="message-date">
                      {new Date(result.message.date).toLocaleDateString()}
                    </span>
                  )}
                </div>
              </div>
              <div className="message-text">
                {result.message?.text || result.text_preview}
              </div>
              {result.message && (
                <div className="message-badges">
                  {result.message.sentiment_label && (
                    <span className={`badge badge-${result.message.sentiment_label}`}>
                      {result.message.sentiment_label}
                    </span>
                  )}
                  {result.message.is_toxic && <span className="badge badge-toxic">Toxic</span>}
                  {result.message.is_humorous && <span className="badge badge-humor">Humor</span>}
                </div>
              )}
            </div>
          ))}
        </>
      ) : query && !loading ? (
        <div style={{ textAlign: 'center', padding: 'var(--spacing-xl)', color: 'var(--text-muted)' }}>
          No results found for "{query}"
        </div>
      ) : null}

      {/* Help text */}
      {!query && searchAvailable && (
        <div style={{ textAlign: 'center', padding: 'var(--spacing-xl)', color: 'var(--text-muted)' }}>
          <p style={{ marginBottom: 'var(--spacing-sm)' }}>
            Enter a search query to find semantically similar messages
          </p>
          <p style={{ fontSize: '12px' }}>
            The search converts your query to an embedding and finds messages with similar meaning
          </p>
        </div>
      )}
    </div>
  )
}
