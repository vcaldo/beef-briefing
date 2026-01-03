import { useState, useEffect, useCallback } from 'react'
import { useLaunchParams, backButton } from '@telegram-apps/sdk-react'

import { apiClient, CardImageWithUrl } from './api/client'
import { WeekSelector } from './components/WeekSelector'
import { CardGallery } from './components/CardGallery'

type AppState = 'loading' | 'authenticated' | 'error'

function App() {
  const launchParams = useLaunchParams()

  const [state, setState] = useState<AppState>('loading')
  const [error, setError] = useState<string | null>(null)
  const [weeks, setWeeks] = useState<string[]>([])
  const [selectedWeek, setSelectedWeek] = useState<string | null>(null)
  const [cards, setCards] = useState<CardImageWithUrl[]>([])
  const [isLoadingCards, setIsLoadingCards] = useState(false)
  const [selectedCard, setSelectedCard] = useState<CardImageWithUrl | null>(null)

  // Handle back button for card zoom
  useEffect(() => {
    if (backButton.show.isAvailable()) {
      if (selectedCard) {
        backButton.show()
      } else {
        backButton.hide()
      }
    }
  }, [selectedCard])

  useEffect(() => {
    if (!backButton.onClick.isAvailable()) return

    const off = backButton.onClick(() => {
      if (selectedCard) {
        setSelectedCard(null)
      }
    })
    return () => off()
  }, [selectedCard])

  // Authenticate on mount
  useEffect(() => {
    async function init() {
      try {
        const initDataRaw = launchParams.initDataRaw
        if (!initDataRaw) {
          // Development fallback - check for mock mode
          if (import.meta.env.DEV) {
            setError('No init data available. Running in development mode without Telegram context.')
          } else {
            setError('This app must be opened from Telegram')
          }
          setState('error')
          return
        }

        const auth = await apiClient.authenticate(initDataRaw)

        if (!auth.chat_id) {
          setError('Please open this app from a group chat using the /deck command')
          setState('error')
          return
        }

        // Fetch available weeks
        const availableWeeks = await apiClient.getWeeks(auth.chat_id)

        if (availableWeeks.length === 0) {
          setError('No card data available for this group yet')
          setState('error')
          return
        }

        setWeeks(availableWeeks)
        setSelectedWeek(availableWeeks[0])
        setState('authenticated')
      } catch (err) {
        console.error('Authentication error:', err)
        setError(err instanceof Error ? err.message : 'Failed to authenticate')
        setState('error')
      }
    }

    init()
  }, [launchParams.initDataRaw])

  // Fetch cards when week changes
  useEffect(() => {
    if (state !== 'authenticated' || !selectedWeek) return

    async function fetchCards() {
      setIsLoadingCards(true)
      try {
        const cardData = await apiClient.getCardsWithUrls(selectedWeek!)
        setCards(cardData)
      } catch (err) {
        console.error('Failed to load cards:', err)
      } finally {
        setIsLoadingCards(false)
      }
    }

    fetchCards()
  }, [state, selectedWeek])

  const handleWeekSelect = useCallback((week: string) => {
    setSelectedWeek(week)
    setSelectedCard(null)
  }, [])

  const handleCardClick = useCallback((card: CardImageWithUrl) => {
    setSelectedCard(card)
  }, [])

  if (state === 'loading') {
    return (
      <div className="app loading-container">
        <div className="loading-spinner">
          <div className="spinner"></div>
          <p>Loading...</p>
        </div>
      </div>
    )
  }

  if (state === 'error') {
    return (
      <div className="app error-container">
        <div className="error-content">
          <div className="error-icon">!</div>
          <p>{error}</p>
        </div>
      </div>
    )
  }

  // Card zoom overlay
  if (selectedCard) {
    return (
      <div className="app">
        <div className="card-zoom-overlay" onClick={() => setSelectedCard(null)}>
          <div className="card-zoom-content" onClick={(e) => e.stopPropagation()}>
            <img src={selectedCard.url} alt={selectedCard.first_name || 'Card'} />
            <div className="card-zoom-info">
              <span className="card-zoom-name">
                {[selectedCard.first_name, selectedCard.last_name].filter(Boolean).join(' ')}
              </span>
              {selectedCard.username && (
                <span className="card-zoom-username">@{selectedCard.username}</span>
              )}
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>Deck Gallery</h1>
      </header>

      {weeks.length > 0 && (
        <WeekSelector
          weeks={weeks}
          selected={selectedWeek}
          onSelect={handleWeekSelect}
        />
      )}

      {isLoadingCards ? (
        <div className="loading-container">
          <div className="spinner"></div>
        </div>
      ) : (
        <CardGallery cards={cards} onCardClick={handleCardClick} />
      )}
    </div>
  )
}

export default App
