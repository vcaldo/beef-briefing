import { useState, useEffect } from 'react'

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
      </div>
    </div>
  )
}
