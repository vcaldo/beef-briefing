import { useState, useEffect, useCallback } from 'react'
import { shareStory, openLink } from '@telegram-apps/sdk-react'

import { apiClient } from '../../api/client'
import type { CardImageWithUrl } from '../../types'

interface CardPageProps {
  userId: number
  chatTitle: string | null
}

export function CardPage({ userId, chatTitle }: CardPageProps) {
  const [card, setCard] = useState<CardImageWithUrl | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    async function fetchCard() {
      setIsLoading(true)
      setError(null)
      try {
        const userCard = await apiClient.getUserLatestCard(userId)
        setCard(userCard)
      } catch (err) {
        console.error('Failed to fetch card:', err)
        setError(err instanceof Error ? err.message : 'Failed to load card')
      } finally {
        setIsLoading(false)
      }
    }

    fetchCard()
  }, [userId])

  const handleShareToStory = useCallback(() => {
    if (!card) return
    shareStory(card.url, {
      text: 'Check out my Deck card!',
    })
  }, [card])

  const handleDownload = useCallback(() => {
    if (!card) return
    if (openLink.isAvailable()) {
      openLink(card.url)
    } else {
      window.open(card.url, '_blank')
    }
  }, [card])

  // Format week date for display
  const formatWeekDate = (weekStart: string) => {
    const date = new Date(weekStart)
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
  }

  if (isLoading) {
    return (
      <div className="page-container">
        <header className="app-header">
          <h1>My Card</h1>
          {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
        </header>
        <div className="loading-container">
          <div className="spinner" />
          <p>Loading your card...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="page-container">
        <header className="app-header">
          <h1>My Card</h1>
          {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
        </header>
        <div className="card-empty-state">
          <div className="card-empty-icon">!</div>
          <p>{error}</p>
        </div>
      </div>
    )
  }

  if (!card) {
    return (
      <div className="page-container">
        <header className="app-header">
          <h1>My Card</h1>
          {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
        </header>
        <div className="card-empty-state">
          <div className="card-empty-icon">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
              <circle cx="8.5" cy="8.5" r="1.5" />
              <polyline points="21 15 16 10 5 21" />
            </svg>
          </div>
          <p className="card-empty-title">No card available yet</p>
          <p className="card-empty-subtitle">Your card will appear here after the weekly stats are generated</p>
        </div>
      </div>
    )
  }

  return (
    <div className="page-container">
      <header className="app-header">
        <h1>My Card</h1>
        {chatTitle && <p className="app-header-subtitle">{chatTitle}</p>}
      </header>

      <div className="card-display-section">
        <div className="card-image-container">
          <img
            src={card.url}
            alt="Your weekly card"
            className="card-image"
          />
        </div>

        <div className="card-actions">
          {shareStory.isAvailable() && (
            <button className="card-action-btn card-action-primary" onClick={handleShareToStory}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10" />
                <circle cx="12" cy="12" r="3" />
              </svg>
              Share to Story
            </button>
          )}
          <button className="card-action-btn" onClick={handleDownload}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="7 10 12 15 17 10" />
              <line x1="12" y1="15" x2="12" y2="3" />
            </svg>
            Download
          </button>
        </div>
      </div>
    </div>
  )
}
