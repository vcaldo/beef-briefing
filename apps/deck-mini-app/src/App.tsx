import { useState, useEffect, useCallback, useRef } from 'react'
import { useLaunchParams, backButton, shareStory, cloudStorage, openLink } from '@telegram-apps/sdk-react'

import { apiClient, CardImageWithUrl } from './api/client'
import { setCustomAttribute, addPageAction, noticeError } from '@beef-briefing/shared-mini-app/monitoring'
import { WeekSelector } from './components/WeekSelector'
import { CardGallery } from './components/CardGallery'
import { InfoModal } from './components/InfoModal'
import { CardReveal } from './components/CardReveal'

const REVEAL_SEEN_KEY = 'card_reveal_seen_v1'

function useCardRevealState(weekStart: string | null) {
  const [hasSeenReveal, setHasSeenReveal] = useState<boolean | null>(null)
  const [isCheckingStorage, setIsCheckingStorage] = useState(true)

  useEffect(() => {
    if (!weekStart) return

    const checkStorage = async () => {
      try {
        if (!cloudStorage.getItem.isAvailable()) {
          setHasSeenReveal(false)
          setIsCheckingStorage(false)
          return
        }

        const stored = await cloudStorage.getItem(REVEAL_SEEN_KEY)
        if (stored) {
          const seen = JSON.parse(stored) as Record<string, boolean>
          setHasSeenReveal(seen[weekStart] === true)
        } else {
          setHasSeenReveal(false)
        }
      } catch (error) {
        console.error('Failed to check CloudStorage:', error)
        setHasSeenReveal(false)
      } finally {
        setIsCheckingStorage(false)
      }
    }

    checkStorage()
  }, [weekStart])

  const markAsSeen = useCallback(async (week: string) => {
    // Update local state immediately to prevent re-triggering
    setHasSeenReveal(true)

    try {
      if (!cloudStorage.setItem.isAvailable()) return

      let seen: Record<string, boolean> = {}
      if (cloudStorage.getItem.isAvailable()) {
        const stored = await cloudStorage.getItem(REVEAL_SEEN_KEY)
        if (stored) {
          seen = JSON.parse(stored)
        }
      }

      seen[week] = true

      // Keep only last 10 weeks
      const keys = Object.keys(seen).sort().reverse()
      if (keys.length > 10) {
        keys.slice(10).forEach((k) => delete seen[k])
      }

      await cloudStorage.setItem(REVEAL_SEEN_KEY, JSON.stringify(seen))
    } catch (error) {
      console.error('Failed to save to CloudStorage:', error)
    }
  }, [])

  const resetReveal = useCallback(async () => {
    setHasSeenReveal(false)
    try {
      if (cloudStorage.deleteItem.isAvailable()) {
        await cloudStorage.deleteItem(REVEAL_SEEN_KEY)
      }
    } catch (error) {
      console.error('Failed to reset CloudStorage:', error)
    }
  }, [])

  return { hasSeenReveal, isCheckingStorage, markAsSeen, resetReveal }
}

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
  const [swipeOffset, setSwipeOffset] = useState(0)
  const [isAnimating, setIsAnimating] = useState(false)
  const [currentUserId, setCurrentUserId] = useState<number | null>(null)
  const [showReveal, setShowReveal] = useState(false)
  const [revealCard, setRevealCard] = useState<CardImageWithUrl | null>(null)

  // Derive selected card from index
  const selectedCard = selectedCardIndex !== null ? cards[selectedCardIndex] : null

  // Card reveal state
  const { hasSeenReveal, isCheckingStorage, markAsSeen, resetReveal } = useCardRevealState(selectedWeek)
  const userCard = cards.find((c) => c.user_id === currentUserId)

  // Debug: long-press header to reset reveal (hold 3 seconds)
  const longPressTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const handleHeaderPressStart = useCallback(() => {
    longPressTimer.current = setTimeout(async () => {
      await resetReveal()
      alert('Card reveal reset! Reload to see animation again.')
    }, 3000)
  }, [resetReveal])
  const handleHeaderPressEnd = useCallback(() => {
    if (longPressTimer.current) {
      clearTimeout(longPressTimer.current)
      longPressTimer.current = null
    }
  }, [])

  // Swipe gesture tracking (horizontal for mobile)
  const touchStartX = useRef<number | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)

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

  // Set timezone attribute on mount
  useEffect(() => {
    try {
      const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
      setCustomAttribute('timezone', timezone)
    } catch {
      setCustomAttribute('timezone', 'unknown')
    }
  }, [])

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
          addPageAction('auth_failed', { reason: 'no_init_data' })
          return
        }

        const auth = await apiClient.authenticate(initDataRaw)
        setCurrentUserId(auth.user_id)

        if (!auth.chat_id) {
          setError('Please open this app from a group chat using the /deck command')
          setState('error')
          addPageAction('auth_failed', { reason: 'no_chat_id' })
          return
        }

        // Set New Relic custom attributes
        setCustomAttribute('user_id', auth.user_id)
        setCustomAttribute('chat_id', auth.chat_id)
        setCustomAttribute('is_authenticated', true)

        addPageAction('auth_success', {
          user_id: auth.user_id,
          chat_id: auth.chat_id,
        })

        // Fetch available weeks
        const availableWeeks = await apiClient.getWeeks(auth.chat_id)

        if (availableWeeks.length === 0) {
          setError('No card data available for this group yet')
          setState('error')
          addPageAction('no_weeks_available', { chat_id: auth.chat_id })
          return
        }

        setWeeks(availableWeeks)
        setSelectedWeek(availableWeeks[0])
        setCustomAttribute('selected_week', availableWeeks[0])
        setState('authenticated')
      } catch (err) {
        console.error('Authentication error:', err)
        setError(err instanceof Error ? err.message : 'Failed to authenticate')
        setState('error')

        if (err instanceof Error) {
          noticeError(err, { context: 'authentication' })
        }
        addPageAction('auth_error', {
          error: err instanceof Error ? err.message : 'unknown',
        })
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

  // Trigger card reveal when conditions are met
  useEffect(() => {
    if (
      state === 'authenticated' &&
      !isLoadingCards &&
      cards.length > 0 &&
      userCard &&
      hasSeenReveal === false &&
      !isCheckingStorage &&
      !showReveal
    ) {
      setRevealCard(userCard)
      setShowReveal(true)
    }
  }, [state, isLoadingCards, cards, userCard, hasSeenReveal, isCheckingStorage, showReveal])

  // Handle slow CloudStorage - timeout after 500ms
  useEffect(() => {
    if (
      state === 'authenticated' &&
      !isLoadingCards &&
      cards.length > 0 &&
      userCard &&
      isCheckingStorage &&
      !showReveal
    ) {
      const timeout = setTimeout(() => {
        setRevealCard(userCard)
        setShowReveal(true)
      }, 500)
      return () => clearTimeout(timeout)
    }
  }, [state, isLoadingCards, cards, userCard, isCheckingStorage, showReveal])

  const handleRevealComplete = useCallback(async () => {
    if (selectedWeek) {
      await markAsSeen(selectedWeek)
    }
    addPageAction('card_reveal_complete', {
      week: selectedWeek,
      card_id: revealCard?.id,
    })
    setShowReveal(false)
    setRevealCard(null)
  }, [selectedWeek, markAsSeen, revealCard])

  const handleWeekSelect = useCallback((week: string) => {
    addPageAction('week_selected', { week, previous_week: selectedWeek })
    setCustomAttribute('selected_week', week)
    setSelectedWeek(week)
    setSelectedCardIndex(null)
  }, [selectedWeek])

  const handleCardClick = useCallback((card: CardImageWithUrl) => {
    const index = cards.findIndex(c => c.id === card.id)
    setSelectedCardIndex(index >= 0 ? index : null)
    addPageAction('card_viewed', {
      card_id: card.id,
      user_id: card.user_id,
      is_own_card: card.user_id === currentUserId,
    })
  }, [cards, currentUserId])

  // Navigation functions with wrap-around
  const goToNextCard = useCallback(() => {
    if (selectedCardIndex === null || cards.length === 0) return
    setSelectedCardIndex((selectedCardIndex + 1) % cards.length)
  }, [selectedCardIndex, cards.length])

  const goToPrevCard = useCallback(() => {
    if (selectedCardIndex === null || cards.length === 0) return
    setSelectedCardIndex((selectedCardIndex - 1 + cards.length) % cards.length)
  }, [selectedCardIndex, cards.length])

  // Share to story handler (only for own card)
  const handleShareToStory = useCallback(() => {
    if (!selectedCard) return
    shareStory(selectedCard.url, {
      text: 'Check out my Deck card!',
    })
    addPageAction('share_to_story', {
      card_id: selectedCard.id,
      week: selectedWeek,
    })
  }, [selectedCard, selectedWeek])

  // Handlers for card reveal
  const handleRevealShareStory = useCallback(() => {
    if (!revealCard) return
    shareStory(revealCard.url, {
      text: 'Check out my Deck card!',
    })
    addPageAction('share_to_story', {
      card_id: revealCard.id,
      week: selectedWeek,
      from_reveal: true,
    })
  }, [revealCard, selectedWeek])

  const handleRevealDownload = useCallback(() => {
    if (!revealCard) return
    if (openLink.isAvailable()) {
      openLink(revealCard.url)
    } else {
      window.open(revealCard.url, '_blank')
    }
    addPageAction('card_download', {
      card_id: revealCard.id,
      week: selectedWeek,
      from_reveal: true,
    })
  }, [revealCard, selectedWeek])

  const handleDownload = useCallback(async () => {
    if (!selectedCard) return
    if (openLink.isAvailable()) {
      openLink(selectedCard.url)
    } else {
      window.open(selectedCard.url, '_blank')
    }
    addPageAction('card_download', {
      card_id: selectedCard.id,
      week: selectedWeek,
    })
  }, [selectedCard, selectedWeek])

  // Swipe gesture handlers (horizontal: swipe left = next, swipe right = prev)
  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    if (isAnimating) return
    touchStartX.current = e.touches[0].clientX
  }, [isAnimating])

  const handleTouchMove = useCallback((e: React.TouchEvent) => {
    if (touchStartX.current === null || isAnimating) return
    const diff = e.touches[0].clientX - touchStartX.current
    setSwipeOffset(diff)
  }, [isAnimating])

  const handleTouchEnd = useCallback((e: React.TouchEvent) => {
    if (touchStartX.current === null || isAnimating) return

    const diff = e.changedTouches[0].clientX - touchStartX.current
    const threshold = 50
    const containerWidth = containerRef.current?.offsetWidth || window.innerWidth

    setIsAnimating(true)

    if (diff < -threshold) {
      // Swipe left → next card
      setSwipeOffset(-containerWidth)
      setTimeout(() => {
        goToNextCard()
        setSwipeOffset(0)
        setIsAnimating(false)
      }, 300)
    } else if (diff > threshold) {
      // Swipe right → prev card
      setSwipeOffset(containerWidth)
      setTimeout(() => {
        goToPrevCard()
        setSwipeOffset(0)
        setIsAnimating(false)
      }, 300)
    } else {
      // Below threshold → snap back
      setSwipeOffset(0)
      setTimeout(() => setIsAnimating(false), 300)
    }

    touchStartX.current = null
  }, [isAnimating, goToNextCard, goToPrevCard])

  // Keyboard navigation
  useEffect(() => {
    if (selectedCardIndex === null) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (isAnimating) return
      const containerWidth = containerRef.current?.offsetWidth || window.innerWidth

      if (e.key === 'ArrowLeft') {
        setIsAnimating(true)
        setSwipeOffset(containerWidth)
        setTimeout(() => {
          goToPrevCard()
          setSwipeOffset(0)
          setIsAnimating(false)
        }, 300)
      }
      if (e.key === 'ArrowRight') {
        setIsAnimating(true)
        setSwipeOffset(-containerWidth)
        setTimeout(() => {
          goToNextCard()
          setSwipeOffset(0)
          setIsAnimating(false)
        }, 300)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [selectedCardIndex, isAnimating, goToPrevCard, goToNextCard])

  // Preload adjacent card images
  useEffect(() => {
    if (selectedCardIndex === null || cards.length === 0) return

    const preload = (url: string) => {
      const img = new Image()
      img.src = url
    }

    const prevIndex = (selectedCardIndex - 1 + cards.length) % cards.length
    const nextIndex = (selectedCardIndex + 1) % cards.length

    preload(cards[prevIndex].url)
    preload(cards[nextIndex].url)
  }, [selectedCardIndex, cards])

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

  // Card reveal animation
  if (showReveal && revealCard && selectedWeek) {
    const cardName = [revealCard.first_name, revealCard.last_name].filter(Boolean).join(' ')
    return (
      <div className="app">
        <CardReveal
          cardUrl={revealCard.url}
          cardName={cardName}
          onComplete={handleRevealComplete}
          onShareStory={handleRevealShareStory}
          onDownload={handleRevealDownload}
          showShareStory={shareStory.isAvailable()}
        />
      </div>
    )
  }

  // Card zoom overlay with navigation
  if (selectedCard) {
    const prevIndex = (selectedCardIndex! - 1 + cards.length) % cards.length
    const nextIndex = (selectedCardIndex! + 1) % cards.length

    return (
      <div className="app">
        <div className="card-zoom-overlay" onClick={() => setSelectedCardIndex(null)}>
          <div
            ref={containerRef}
            className="card-carousel-track"
            style={{
              transform: `translate3d(${swipeOffset}px, 0, 0)`,
              transition: isAnimating ? 'transform 0.3s ease-out' : 'none'
            }}
            onTouchStart={handleTouchStart}
            onTouchMove={handleTouchMove}
            onTouchEnd={handleTouchEnd}
          >
            {/* Previous card */}
            <div className="card-carousel-item card-prev">
              <img src={cards[prevIndex].url} alt="" />
            </div>

            {/* Current card */}
            <div className="card-carousel-item card-current" onClick={(e) => e.stopPropagation()}>
              <img src={selectedCard.url} alt={selectedCard.first_name || 'Card'} />
              {currentUserId === selectedCard.user_id && (
                <div className="card-actions">
                  {shareStory.isAvailable() && (
                    <button className="card-action-btn" onClick={(e) => { e.stopPropagation(); handleShareToStory(); }}>
                      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <circle cx="12" cy="12" r="10"/>
                        <circle cx="12" cy="12" r="3"/>
                      </svg>
                      Story
                    </button>
                  )}
                  <button className="card-action-btn" onClick={(e) => { e.stopPropagation(); handleDownload(); }}>
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                      <polyline points="7 10 12 15 17 10"/>
                      <line x1="12" y1="15" x2="12" y2="3"/>
                    </svg>
                    Download
                  </button>
                </div>
              )}
            </div>

            {/* Next card */}
            <div className="card-carousel-item card-next">
              <img src={cards[nextIndex].url} alt="" />
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="app">
      <header
        className="app-header"
        onMouseDown={handleHeaderPressStart}
        onMouseUp={handleHeaderPressEnd}
        onMouseLeave={handleHeaderPressEnd}
        onTouchStart={handleHeaderPressStart}
        onTouchEnd={handleHeaderPressEnd}
      >
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
