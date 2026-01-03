import { useState, useEffect, useCallback, useRef } from 'react'
import { useLaunchParams, backButton } from '@telegram-apps/sdk-react'

import { apiClient, CardImageWithUrl } from './api/client'
import { WeekSelector } from './components/WeekSelector'
import { CardGallery } from './components/CardGallery'
import { InfoModal } from './components/InfoModal'

type AppState = 'loading' | 'authenticated' | 'error'

function App() {
  const launchParams = useLaunchParams()

  const [state, setState] = useState<AppState>('loading')
  const [error, setError] = useState<string | null>(null)
  const [weeks, setWeeks] = useState<string[]>([])
  const [selectedWeek, setSelectedWeek] = useState<string | null>(null)
  const [cards, setCards] = useState<CardImageWithUrl[]>([])
  const [isLoadingCards, setIsLoadingCards] = useState(false)
  const [selectedCardIndex, setSelectedCardIndex] = useState<number | null>(null)
  const [isInfoOpen, setIsInfoOpen] = useState(false)

  // Derive selected card from index
  const selectedCard = selectedCardIndex !== null ? cards[selectedCardIndex] : null

  // Swipe gesture tracking (horizontal for mobile)
  const touchStartX = useRef<number | null>(null)

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
        setSelectedCardIndex(null)
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
    setSelectedCardIndex(null)
  }, [])

  const handleCardClick = useCallback((card: CardImageWithUrl) => {
    const index = cards.findIndex(c => c.id === card.id)
    setSelectedCardIndex(index >= 0 ? index : null)
  }, [cards])

  // Navigation functions with wrap-around
  const goToNextCard = useCallback(() => {
    if (selectedCardIndex === null || cards.length === 0) return
    setSelectedCardIndex((selectedCardIndex + 1) % cards.length)
  }, [selectedCardIndex, cards.length])

  const goToPrevCard = useCallback(() => {
    if (selectedCardIndex === null || cards.length === 0) return
    setSelectedCardIndex((selectedCardIndex - 1 + cards.length) % cards.length)
  }, [selectedCardIndex, cards.length])

  // Swipe gesture handlers (horizontal: swipe left = next, swipe right = prev)
  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    touchStartX.current = e.touches[0].clientX
  }, [])

  const handleTouchEnd = useCallback((e: React.TouchEvent) => {
    if (touchStartX.current === null) return
    const diff = e.changedTouches[0].clientX - touchStartX.current
    const threshold = 50
    if (diff < -threshold) goToNextCard()  // swipe left = next
    else if (diff > threshold) goToPrevCard()  // swipe right = prev
    touchStartX.current = null
  }, [goToPrevCard, goToNextCard])

  // Keyboard navigation
  useEffect(() => {
    if (selectedCardIndex === null) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft') goToPrevCard()
      if (e.key === 'ArrowRight') goToNextCard()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [selectedCardIndex, goToPrevCard, goToNextCard])

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

  // Card zoom overlay with navigation
  if (selectedCard) {
    return (
      <div className="app">
        <div
          className="card-zoom-overlay"
          onClick={() => setSelectedCardIndex(null)}
          onTouchStart={handleTouchStart}
          onTouchEnd={handleTouchEnd}
        >
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
        <button className="info-btn" onClick={() => setIsInfoOpen(true)} aria-label="Info">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="16" x2="12" y2="12"/>
            <line x1="12" y1="8" x2="12.01" y2="8"/>
          </svg>
        </button>
      </header>

      <InfoModal isOpen={isInfoOpen} onClose={() => setIsInfoOpen(false)} />

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
