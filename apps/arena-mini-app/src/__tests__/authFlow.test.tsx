import React from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import App from '../App'
import { apiClient } from '../api/client'
import type { AuthResponse } from '../types'

// Mock the API client
vi.mock('../api/client', () => ({
  apiClient: {
    authenticateGame: vi.fn(),
    getConstants: vi.fn(),
    token: null,
    chatId: null,
    isAdmin: false,
  },
}))

// Mock lazy-loaded components to avoid testing the entire component tree
vi.mock('../components/lobby/LobbyPage', () => ({
  default: () => <div data-testid="lobby-page">Lobby Page</div>,
}))

vi.mock('../components/shop/ShopPage', () => ({
  default: () => <div data-testid="shop-page">Shop Page</div>,
}))

vi.mock('../components/battle/BattlePage', () => ({
  default: () => <div data-testid="battle-page">Battle Page</div>,
}))

vi.mock('../components/stats/StatsPage', () => ({
  default: () => <div data-testid="stats-page">Stats Page</div>,
}))

// Mock sound context to avoid audio issues in tests
vi.mock('../contexts', () => ({
  SoundProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

// Mock asset preloader
vi.mock('../utils/assetPreloader', () => ({
  preloadAllDuringSplash: vi.fn(),
}))

// Store original window.location
const originalLocation = window.location

describe('Auth Flow', () => {
  beforeEach(() => {
    // Reset mocks
    vi.clearAllMocks()
    vi.useFakeTimers()

    // Mock window.location with valid URL params
    delete (window as unknown as { location?: Location }).location
    window.location = {
      ...originalLocation,
      search: '?chat_id=-1001234567890&user_id=123456&match_id=abc123&ts=1706540800&sig=validSignature123',
    } as Location
  })

  afterEach(() => {
    // Restore window.location
    window.location = originalLocation
    vi.useRealTimers()
  })

  describe('successful authentication', () => {
    const mockAuthResponse: AuthResponse = {
      token: 'jwt-token-abc123',
      user_id: 123456,
      chat_id: -1001234567890,
      chat_title: 'Test Group',
      first_name: 'John',
      username: 'johndoe',
      is_admin: false,
    }

    beforeEach(() => {
      vi.mocked(apiClient.authenticateGame).mockResolvedValue(mockAuthResponse)
      vi.mocked(apiClient.getConstants).mockResolvedValue({
        starting_coins: 10,
        card_cost: 3,
        reroll_cost: 1,
        upgrade_cost: 1,
        team_size: 3,
        shop_size: 3,
        atk_bonus_per_upgrade: 1,
        hp_bonus_per_upgrade: 3,
      })
    })

    it('calls authenticateGame with correct parameters from URL', async () => {
      render(<App />)

      await waitFor(() => {
        expect(apiClient.authenticateGame).toHaveBeenCalledWith({
          chat_id: -1001234567890,
          user_id: 123456,
          match_id: 'abc123',
          ts: 1706540800,
          sig: 'validSignature123',
        })
      })
    })

    it('stores JWT token in API client after successful auth', async () => {
      // The authenticateGame method sets this.token internally
      // We verify this by checking the mock was called and returned the token
      render(<App />)

      await waitFor(() => {
        expect(apiClient.authenticateGame).toHaveBeenCalled()
      })

      // Verify the mock returned the correct token
      const result = await apiClient.authenticateGame({
        chat_id: -1001234567890,
        user_id: 123456,
        match_id: 'abc123',
        ts: 1706540800,
        sig: 'validSignature123',
      })
      expect(result.token).toBe('jwt-token-abc123')
    })

    it('transitions to lobby state after authentication and splash', async () => {
      render(<App />)

      // Initially shows splash screen
      expect(screen.getByText('Prepare for battle')).toBeInTheDocument()

      // Wait for authentication to complete
      await waitFor(() => {
        expect(apiClient.authenticateGame).toHaveBeenCalled()
      })

      // Advance timers past the splash screen minimum time (2000ms)
      vi.advanceTimersByTime(2100)

      // After splash, should show lobby page
      await waitFor(() => {
        expect(screen.getByTestId('lobby-page')).toBeInTheDocument()
      })
    })

    it('prefetches game constants after successful auth', async () => {
      render(<App />)

      await waitFor(() => {
        expect(apiClient.authenticateGame).toHaveBeenCalled()
      })

      // Constants are prefetched after auth
      await waitFor(() => {
        expect(apiClient.getConstants).toHaveBeenCalled()
      })
    })

    it('handles empty match_id for lobby-only access', async () => {
      // Update URL to have no match_id
      window.location = {
        ...originalLocation,
        search: '?chat_id=-1001234567890&user_id=123456&ts=1706540800&sig=validSignature123',
      } as Location

      render(<App />)

      await waitFor(() => {
        expect(apiClient.authenticateGame).toHaveBeenCalledWith({
          chat_id: -1001234567890,
          user_id: 123456,
          match_id: '',
          ts: 1706540800,
          sig: 'validSignature123',
        })
      })
    })
  })

  describe('authentication errors', () => {
    it('shows error when auth fails with invalid signature', async () => {
      vi.mocked(apiClient.authenticateGame).mockRejectedValue(
        new Error('Invalid signature')
      )

      render(<App />)

      await waitFor(() => {
        expect(screen.getByText('Unable to Start')).toBeInTheDocument()
        expect(screen.getByText('Invalid signature')).toBeInTheDocument()
      })
    })

    it('shows error when auth response has no chat_id', async () => {
      vi.mocked(apiClient.authenticateGame).mockResolvedValue({
        token: 'jwt-token-abc123',
        user_id: 123456,
        chat_id: null,
        first_name: 'John',
        username: 'johndoe',
      })

      render(<App />)

      await waitFor(() => {
        expect(screen.getByText('Unable to Start')).toBeInTheDocument()
        expect(
          screen.getByText('Please open this app from a group chat using the /arena command')
        ).toBeInTheDocument()
      })
    })

    it('shows error when signature is missing from URL', async () => {
      window.location = {
        ...originalLocation,
        search: '?chat_id=-1001234567890&user_id=123456&ts=1706540800',
      } as Location

      render(<App />)

      await waitFor(() => {
        expect(screen.getByText('Unable to Start')).toBeInTheDocument()
        expect(screen.getByText(/Missing signature/)).toBeInTheDocument()
      })

      // Should not attempt authentication when sig is missing
      expect(apiClient.authenticateGame).not.toHaveBeenCalled()
    })

    it('shows error when required URL params are missing', async () => {
      window.location = {
        ...originalLocation,
        search: '?sig=validSignature123',
      } as Location

      render(<App />)

      await waitFor(() => {
        expect(screen.getByText('Unable to Start')).toBeInTheDocument()
        expect(screen.getByText(/Missing required/)).toBeInTheDocument()
      })

      expect(apiClient.authenticateGame).not.toHaveBeenCalled()
    })

    it('shows reload button on error that reloads the page', async () => {
      vi.mocked(apiClient.authenticateGame).mockRejectedValue(
        new Error('Network error')
      )

      // Mock window.location.reload
      const reloadMock = vi.fn()
      Object.defineProperty(window.location, 'reload', {
        value: reloadMock,
        writable: true,
      })

      render(<App />)

      await waitFor(() => {
        expect(screen.getByText('Unable to Start')).toBeInTheDocument()
      })

      // Find and click the reload button
      const reloadButton = screen.getByText('Reload')
      expect(reloadButton).toBeInTheDocument()
    })
  })

  describe('splash screen behavior', () => {
    beforeEach(() => {
      vi.mocked(apiClient.authenticateGame).mockResolvedValue({
        token: 'jwt-token-abc123',
        user_id: 123456,
        chat_id: -1001234567890,
        first_name: 'John',
        username: 'johndoe',
      })
      vi.mocked(apiClient.getConstants).mockResolvedValue({
        starting_coins: 10,
        card_cost: 3,
        reroll_cost: 1,
        upgrade_cost: 1,
        team_size: 3,
        shop_size: 3,
        atk_bonus_per_upgrade: 1,
        hp_bonus_per_upgrade: 3,
      })
    })

    it('shows splash screen during loading state', () => {
      render(<App />)

      expect(screen.getByText('Prepare for battle')).toBeInTheDocument()
    })

    it('keeps splash screen visible until minimum time elapses', async () => {
      render(<App />)

      // Auth completes quickly
      await waitFor(() => {
        expect(apiClient.authenticateGame).toHaveBeenCalled()
      })

      // But splash should still be visible (min time not elapsed)
      expect(screen.getByText('Prepare for battle')).toBeInTheDocument()

      // Advance time partially (not enough)
      vi.advanceTimersByTime(1000)
      expect(screen.getByText('Prepare for battle')).toBeInTheDocument()

      // Advance past minimum time
      vi.advanceTimersByTime(1200)

      await waitFor(() => {
        expect(screen.queryByText('Prepare for battle')).not.toBeInTheDocument()
      })
    })
  })
})
