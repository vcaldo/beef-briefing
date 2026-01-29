import '@testing-library/jest-dom'
import { vi } from 'vitest'

// Mock New Relic monitoring functions
vi.mock('@beef-briefing/shared-mini-app/monitoring', () => ({
  setCustomAttribute: vi.fn(),
  addPageAction: vi.fn(),
  noticeError: vi.fn(),
}))
