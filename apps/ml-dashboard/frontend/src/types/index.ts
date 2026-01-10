/**
 * Type definitions for ML Dashboard.
 */

// Chat
export interface Chat {
  id: number
  title: string
  type: string
  username: string | null
  message_count: number
  processed_count: number
}

// Message with ML results
export interface Message {
  id: number
  message_id: number
  chat_id: number
  user_id: number | null
  text: string
  date: string
  first_name: string | null
  last_name: string | null
  username: string | null
  // Sentiment
  sentiment_label: 'positive' | 'neutral' | 'negative' | null
  score_positive: number | null
  score_neutral: number | null
  score_negative: number | null
  sentiment_confidence: number | null
  // Toxicity
  is_toxic: boolean | null
  toxicity_label: string | null
  toxicity_score: number | null
  // Humor
  is_humorous: boolean | null
  humor_type: string | null
  humor_score: number | null
  // Questions
  is_question: boolean | null
  question_type: string | null
  question_score: number | null
  // Topic
  topic_id: number | null
  topic_similarity: number | null
}

export interface Entity {
  entity_type: string
  entity_text: string
  start_pos: number | null
  end_pos: number | null
  confidence: number | null
}

export interface MessageDetail {
  message: Message
  entities: Entity[]
}

// Messages API response
export interface MessagesResponse {
  messages: Message[]
  total: number
  page_info: {
    limit: number
    offset: number
    has_more: boolean
  }
}

// Topic
export interface Topic {
  topic_id: number
  keywords: string[]
  message_count: number
  actual_count: number
}

export interface TopicsResponse {
  topics: Topic[]
  total_topics: number
  outlier_count: number
}

// User with ML stats
export interface User {
  user_id: number
  first_name: string | null
  last_name: string | null
  username: string | null
  message_count: number
  avg_sentiment: number | null
  toxicity_rate: number | null
  humor_rate: number | null
  question_rate: number | null
}

export interface UsersResponse {
  users: User[]
  total: number
  page_info: {
    limit: number
    offset: number
    has_more: boolean
  }
}

export interface SentimentDistribution {
  counts: {
    total: number
    positive: number
    neutral: number
    negative: number
  }
  percentages: {
    positive: number
    neutral: number
    negative: number
  }
}

export interface EntityMention {
  entity_type: string
  entity_text: string
  count: number
}

export interface UserProfile {
  user_id: number
  sentiment_distribution: SentimentDistribution
  entity_mentions: EntityMention[]
}

export interface UserCard {
  id: number
  user_id: number
  chat_id: number
  week_start: string
  week_end: string
  stats: Record<string, unknown>
  trends: Record<string, unknown>
  image_url: string | null
  created_at: string
}

export interface UserCardsResponse {
  user_id: number
  cards: UserCard[]
  total: number
}

// Search
export interface SearchResult {
  message_id: number
  score: number
  text_preview: string
  message: Message | null
}

export interface SearchResponse {
  results: SearchResult[]
  query: string
  timing: {
    embedding_ms: number
    search_ms: number
  }
}

export interface SearchStatus {
  qdrant: {
    available: boolean
    points_count: number
    status: string
  }
  embedding_model_loaded: boolean
  search_available: boolean
}

// Stats
export interface ProcessingStats {
  total_with_text: number
  processed: number
  sentiment_count: number
  toxicity_count: number
  toxic_count: number
  humor_count: number
  humorous_count: number
  questions_count: number
  question_count: number
  entity_count: number
  topic_count: number
}

export interface StatsResponse {
  processing: ProcessingStats
  qdrant: {
    available: boolean
    points_count: number
    status: string
  }
}

export interface ChatsResponse {
  chats: Chat[]
}

// Filter options
export interface MessageFilters {
  user_id?: number
  sentiment?: 'positive' | 'neutral' | 'negative'
  is_toxic?: boolean
  is_humorous?: boolean
  is_question?: boolean
  topic_id?: number
  sort_by?: 'date' | 'toxicity_score' | 'sentiment_score'
  sort_order?: 'asc' | 'desc'
}
