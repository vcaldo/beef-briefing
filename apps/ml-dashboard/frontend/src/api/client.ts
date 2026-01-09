/**
 * API client for ML Dashboard.
 * No authentication needed - dev-only tool.
 */

import type {
  ChatsResponse,
  StatsResponse,
  MessagesResponse,
  MessageDetail,
  TopicsResponse,
  UsersResponse,
  UserProfile,
  UserCardsResponse,
  SearchResponse,
  SearchStatus,
  MessageFilters,
  Message,
} from '../types'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8052'

class ApiClient {
  private async fetch<T>(url: string, options?: RequestInit): Promise<T> {
    const response = await fetch(`${API_BASE_URL}${url}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ detail: 'Request failed' }))
      throw new Error(error.detail || `HTTP ${response.status}`)
    }

    return response.json()
  }

  // Health & Stats
  async health(): Promise<{ status: string }> {
    return this.fetch('/health')
  }

  async getChats(): Promise<ChatsResponse> {
    return this.fetch('/api/chats')
  }

  async getStats(chatId: number): Promise<StatsResponse> {
    return this.fetch(`/api/stats?chat_id=${chatId}`)
  }

  // Messages
  async getMessages(
    chatId: number,
    limit: number = 50,
    offset: number = 0,
    filters?: MessageFilters
  ): Promise<MessagesResponse> {
    const params = new URLSearchParams({
      chat_id: chatId.toString(),
      limit: limit.toString(),
      offset: offset.toString(),
    })

    if (filters?.user_id) params.set('user_id', filters.user_id.toString())
    if (filters?.sentiment) params.set('sentiment', filters.sentiment)
    if (filters?.is_toxic !== undefined) params.set('is_toxic', filters.is_toxic.toString())
    if (filters?.is_humorous !== undefined) params.set('is_humorous', filters.is_humorous.toString())
    if (filters?.is_question !== undefined) params.set('is_question', filters.is_question.toString())
    if (filters?.topic_id) params.set('topic_id', filters.topic_id.toString())
    if (filters?.sort_by) params.set('sort_by', filters.sort_by)
    if (filters?.sort_order) params.set('sort_order', filters.sort_order)

    return this.fetch(`/api/messages?${params}`)
  }

  async getMessage(messageId: number): Promise<MessageDetail> {
    return this.fetch(`/api/messages/${messageId}`)
  }

  // Search
  async getSearchStatus(): Promise<SearchStatus> {
    return this.fetch('/api/search/status')
  }

  async search(
    query: string,
    chatId?: number,
    userId?: number,
    limit: number = 20
  ): Promise<SearchResponse> {
    return this.fetch('/api/search', {
      method: 'POST',
      body: JSON.stringify({
        query,
        chat_id: chatId,
        user_id: userId,
        limit,
      }),
    })
  }

  // Topics
  async getTopics(chatId: number): Promise<TopicsResponse> {
    return this.fetch(`/api/topics?chat_id=${chatId}`)
  }

  async getTopicMessages(
    chatId: number,
    topicId: number,
    limit: number = 50,
    offset: number = 0
  ): Promise<MessagesResponse> {
    const params = new URLSearchParams({
      chat_id: chatId.toString(),
      limit: limit.toString(),
      offset: offset.toString(),
    })
    return this.fetch(`/api/topics/${topicId}/messages?${params}`)
  }

  // Users
  async getUsers(
    chatId: number,
    limit: number = 50,
    offset: number = 0
  ): Promise<UsersResponse> {
    const params = new URLSearchParams({
      chat_id: chatId.toString(),
      limit: limit.toString(),
      offset: offset.toString(),
    })
    return this.fetch(`/api/users?${params}`)
  }

  async getUserProfile(chatId: number, userId: number): Promise<UserProfile> {
    return this.fetch(`/api/users/${userId}/profile?chat_id=${chatId}`)
  }

  async getUserCards(chatId: number, userId: number): Promise<UserCardsResponse> {
    return this.fetch(`/api/users/${userId}/cards?chat_id=${chatId}`)
  }
}

// Singleton instance
export const api = new ApiClient()
