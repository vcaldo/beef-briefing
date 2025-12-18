# User Cards: Yellow Stats Implementation Plan

**Tier:** 🟡 Medium (Query/Aggregate Existing Data)
**Estimated Complexity:** Medium
**Dependencies:** Existing data + new aggregation logic

---

## Overview

These stats require more sophisticated queries across multiple tables, graph-like analysis of user interactions, and leveraging the embeddings already stored in Qdrant for topic analysis.

---

## Stats to Implement

### 1. Night Owl / Early Bird (Chronotype)
**Source:** `messages.created_at`
**Formula:** Peak activity hours + distribution analysis
**Output:** Time preference label + peak hour

```sql
WITH hourly_activity AS (
    SELECT
        from_user_id as user_id,
        EXTRACT(HOUR FROM created_at) as hour,
        COUNT(*) as message_count
    FROM messages
    WHERE chat_id = $1
    GROUP BY from_user_id, EXTRACT(HOUR FROM created_at)
),
user_peaks AS (
    SELECT
        user_id,
        hour as peak_hour,
        message_count,
        ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY message_count DESC) as rn
    FROM hourly_activity
),
user_distribution AS (
    SELECT
        user_id,
        SUM(CASE WHEN hour BETWEEN 6 AND 11 THEN message_count ELSE 0 END) as morning,
        SUM(CASE WHEN hour BETWEEN 12 AND 17 THEN message_count ELSE 0 END) as afternoon,
        SUM(CASE WHEN hour BETWEEN 18 AND 23 THEN message_count ELSE 0 END) as evening,
        SUM(CASE WHEN hour BETWEEN 0 AND 5 THEN message_count ELSE 0 END) as night,
        SUM(message_count) as total
    FROM hourly_activity
    GROUP BY user_id
)
SELECT
    ud.user_id,
    up.peak_hour,
    CASE
        WHEN ud.night::float / NULLIF(ud.total, 0) > 0.3 THEN 'Night Owl'
        WHEN ud.morning::float / NULLIF(ud.total, 0) > 0.4 THEN 'Early Bird'
        WHEN ud.evening::float / NULLIF(ud.total, 0) > 0.4 THEN 'Evening Person'
        ELSE 'All-Day Chatter'
    END as chronotype,
    ud.morning, ud.afternoon, ud.evening, ud.night
FROM user_distribution ud
JOIN user_peaks up ON ud.user_id = up.user_id AND up.rn = 1;
```

**Labels:**
- "Night Owl" 🦉 (00:00-05:59 dominant)
- "Early Bird" 🐦 (06:00-11:59 dominant)
- "Afternoon Warrior" ⚔️ (12:00-17:59 dominant)
- "Evening Person" 🌙 (18:00-23:59 dominant)
- "All-Day Chatter" 🗣️ (evenly distributed)

---

### 2. Conversation Starter Score
**Source:** `messages` table (reply analysis)
**Formula:** Messages that initiated threads vs replies
**Output:** Ratio + label

```sql
WITH message_threads AS (
    SELECT
        id,
        from_user_id,
        reply_to_message_id,
        CASE
            WHEN reply_to_message_id IS NULL THEN 'starter'
            ELSE 'reply'
        END as message_type
    FROM messages
    WHERE chat_id = $1
),
user_thread_stats AS (
    SELECT
        from_user_id as user_id,
        COUNT(*) FILTER (WHERE message_type = 'starter') as started,
        COUNT(*) FILTER (WHERE message_type = 'reply') as replied,
        COUNT(*) as total
    FROM message_threads
    GROUP BY from_user_id
)
SELECT
    user_id,
    started,
    replied,
    ROUND(100.0 * started / NULLIF(total, 0), 1) as starter_pct,
    CASE
        WHEN started::float / NULLIF(total, 0) > 0.7 THEN 'Initiator'
        WHEN started::float / NULLIF(total, 0) > 0.5 THEN 'Balanced'
        WHEN started::float / NULLIF(total, 0) > 0.3 THEN 'Responder'
        ELSE 'Reactor'
    END as conversation_style
FROM user_thread_stats;
```

**Labels:**
- "Initiator" 🎯 (>70% starters) - Loves starting new topics
- "Balanced" ⚖️ (50-70%) - Equal mix
- "Responder" 💬 (30-50%) - Prefers joining conversations
- "Reactor" 🔄 (<30%) - Mostly replies

---

### 3. Response Time (Speed)
**Source:** `messages` with reply analysis
**Formula:** Average time between a message and user's reply
**Output:** Duration + label

```sql
WITH replies AS (
    SELECT
        m.from_user_id as replier_id,
        m.created_at as reply_time,
        orig.created_at as original_time,
        EXTRACT(EPOCH FROM (m.created_at - orig.created_at)) as response_seconds
    FROM messages m
    JOIN messages orig ON m.reply_to_message_id = orig.id
    WHERE m.chat_id = $1
      AND m.reply_to_message_id IS NOT NULL
      AND m.created_at > orig.created_at  -- Valid reply timing
      AND EXTRACT(EPOCH FROM (m.created_at - orig.created_at)) < 86400  -- Within 24h
)
SELECT
    replier_id as user_id,
    COUNT(*) as reply_count,
    ROUND(AVG(response_seconds)) as avg_response_seconds,
    ROUND(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY response_seconds)) as median_response_seconds,
    CASE
        WHEN AVG(response_seconds) < 60 THEN 'Lightning'
        WHEN AVG(response_seconds) < 300 THEN 'Quick'
        WHEN AVG(response_seconds) < 1800 THEN 'Moderate'
        WHEN AVG(response_seconds) < 7200 THEN 'Thoughtful'
        ELSE 'Contemplative'
    END as speed_label
FROM replies
GROUP BY replier_id
HAVING COUNT(*) >= 5;  -- Minimum sample size
```

**Labels:**
- "Lightning" ⚡ (<1 min avg)
- "Quick" 🏃 (1-5 min)
- "Moderate" 🚶 (5-30 min)
- "Thoughtful" 🤔 (30min-2h)
- "Contemplative" 🧘 (>2h)

---

### 4. Influence Score
**Source:** `messages`, `message_reactions`, reply chains
**Formula:** Composite of reactions received + replies generated + unique engagers
**Output:** Normalized score (0-100)

```sql
WITH user_influence AS (
    SELECT
        m.from_user_id as user_id,
        COUNT(DISTINCT m.id) as message_count,
        COUNT(DISTINCT mr.id) as reactions_received,
        COUNT(DISTINCT mr.user_id) as unique_reactors,
        COUNT(DISTINCT replies.id) as replies_received,
        COUNT(DISTINCT replies.from_user_id) as unique_repliers
    FROM messages m
    LEFT JOIN message_reactions mr ON m.id = mr.message_id
    LEFT JOIN messages replies ON replies.reply_to_message_id = m.id
    WHERE m.chat_id = $1
    GROUP BY m.from_user_id
),
influence_scores AS (
    SELECT
        user_id,
        message_count,
        reactions_received,
        replies_received,
        unique_reactors + unique_repliers as unique_engagers,
        -- Engagement rate (reactions + replies per message)
        (reactions_received + replies_received)::float / NULLIF(message_count, 0) as engagement_rate,
        -- Reach (unique people who engaged)
        (unique_reactors + unique_repliers)::float as reach
    FROM user_influence
)
SELECT
    user_id,
    message_count,
    reactions_received,
    replies_received,
    unique_engagers,
    ROUND(engagement_rate, 2) as engagement_rate,
    -- Normalized influence: 40% engagement rate + 30% reach + 30% volume (log scaled)
    ROUND(
        LEAST(100, (
            40 * (engagement_rate / NULLIF(MAX(engagement_rate) OVER(), 0)) +
            30 * (reach / NULLIF(MAX(reach) OVER(), 0)) +
            30 * (LN(message_count + 1) / NULLIF(MAX(LN(message_count + 1)) OVER(), 0))
        )),
        1
    ) as influence_score
FROM influence_scores;
```

**Labels:**
- 80-100: "Influencer" 👑
- 60-79: "Respected" 🌟
- 40-59: "Active" 💪
- 20-39: "Participant" 🙋
- 0-19: "Observer" 👀

---

### 5. Topic Expert
**Source:** Qdrant embeddings + clustering
**Formula:** Dominant topic clusters for user's messages
**Output:** Top 3 topics with confidence

**Implementation (Python):**

```python
from qdrant_client import QdrantClient
from sklearn.cluster import HDBSCAN
import numpy as np

async def get_user_topics(user_id: int, chat_id: int, qdrant: QdrantClient) -> list[dict]:
    # 1. Fetch user's embeddings from Qdrant
    user_points = qdrant.scroll(
        collection_name="message_embeddings",
        scroll_filter={
            "must": [
                {"key": "chat_id", "match": {"value": chat_id}},
                {"key": "user_id", "match": {"value": user_id}}
            ]
        },
        with_vectors=True,
        limit=1000
    )[0]

    if len(user_points) < 10:
        return []

    # 2. Get all chat embeddings for context
    all_points = qdrant.scroll(
        collection_name="message_embeddings",
        scroll_filter={"must": [{"key": "chat_id", "match": {"value": chat_id}}]},
        with_vectors=True,
        limit=10000
    )[0]

    # 3. Cluster all messages
    all_vectors = np.array([p.vector for p in all_points])
    clusterer = HDBSCAN(min_cluster_size=10, metric='euclidean')
    labels = clusterer.fit_predict(all_vectors)

    # 4. Map user messages to clusters
    user_point_ids = {p.id for p in user_points}
    user_clusters = []
    for i, point in enumerate(all_points):
        if point.id in user_point_ids and labels[i] != -1:
            user_clusters.append(labels[i])

    # 5. Count user's cluster distribution
    from collections import Counter
    cluster_counts = Counter(user_clusters)
    total = sum(cluster_counts.values())

    # 6. Get top 3 topics with representative keywords
    top_topics = []
    for cluster_id, count in cluster_counts.most_common(3):
        # Get representative messages for this cluster
        cluster_texts = [
            all_points[i].payload.get('text_preview', '')
            for i, label in enumerate(labels) if label == cluster_id
        ][:10]

        top_topics.append({
            "topic_id": cluster_id,
            "confidence": round(count / total, 2),
            "message_count": count,
            "sample_texts": cluster_texts[:3]
        })

    return top_topics
```

**Topic Naming (Optional - LLM-based):**
```python
async def name_topic(sample_texts: list[str]) -> str:
    # Use small LLM to generate topic name from samples
    prompt = f"Based on these messages, give a 2-3 word topic name:\n{sample_texts}"
    # Call LLM API
    return topic_name
```

---

### 6. Social Graph Metrics
**Source:** `messages`, `message_reactions`
**Formula:** Who talks to whom, reply/reaction networks
**Output:** Bridge vs Insider score

```sql
WITH interactions AS (
    -- Replies
    SELECT
        m.from_user_id as user_a,
        orig.from_user_id as user_b,
        'reply' as interaction_type
    FROM messages m
    JOIN messages orig ON m.reply_to_message_id = orig.id
    WHERE m.chat_id = $1 AND m.from_user_id != orig.from_user_id

    UNION ALL

    -- Reactions
    SELECT
        mr.user_id as user_a,
        m.from_user_id as user_b,
        'reaction' as interaction_type
    FROM message_reactions mr
    JOIN messages m ON mr.message_id = m.id
    WHERE m.chat_id = $1 AND mr.user_id != m.from_user_id
),
user_connections AS (
    SELECT
        user_a as user_id,
        COUNT(DISTINCT user_b) as unique_connections,
        COUNT(*) as total_interactions
    FROM interactions
    GROUP BY user_a
),
chat_stats AS (
    SELECT
        COUNT(DISTINCT from_user_id) as total_users
    FROM messages
    WHERE chat_id = $1
)
SELECT
    uc.user_id,
    uc.unique_connections,
    uc.total_interactions,
    ROUND(100.0 * uc.unique_connections / cs.total_users, 1) as network_reach_pct,
    CASE
        WHEN uc.unique_connections::float / cs.total_users > 0.5 THEN 'Bridge'
        WHEN uc.unique_connections::float / cs.total_users > 0.25 THEN 'Connector'
        WHEN uc.unique_connections > 5 THEN 'Cluster Member'
        ELSE 'Peripheral'
    END as network_role
FROM user_connections uc, chat_stats cs;
```

**Labels:**
- "Bridge" 🌉 (>50% connections) - Connects different groups
- "Connector" 🔗 (25-50%) - Well-connected
- "Cluster Member" 👥 (5+ connections, <25%) - Part of a subgroup
- "Peripheral" 🏝️ (<5 connections) - Loosely connected

---

## Database Schema Additions

```sql
-- Add to ml_user_card_stats table
ALTER TABLE ml_user_card_stats ADD COLUMN IF NOT EXISTS
    -- Yellow stats
    chronotype VARCHAR(20),
    peak_hour SMALLINT,
    morning_pct NUMERIC(5,2),
    evening_pct NUMERIC(5,2),

    conversation_starter_pct NUMERIC(5,2),
    conversation_style VARCHAR(20),
    threads_started INTEGER,
    replies_sent INTEGER,

    avg_response_seconds INTEGER,
    median_response_seconds INTEGER,
    speed_label VARCHAR(20),

    influence_score NUMERIC(5,1),
    influence_label VARCHAR(20),
    engagement_rate NUMERIC(5,2),
    unique_engagers INTEGER,

    top_topics JSONB,  -- Array of {topic_id, confidence, keywords}

    network_reach_pct NUMERIC(5,2),
    network_role VARCHAR(20),
    unique_connections INTEGER;
```

---

## Implementation Steps

### Step 1: Extend Database Schema
- Add yellow stat columns to `ml_user_card_stats`
- Create supporting indexes

### Step 2: Aggregation Queries
Create PostgreSQL functions for each stat:

```sql
CREATE OR REPLACE FUNCTION calculate_chronotype(user_id_param BIGINT, chat_id_param BIGINT)
RETURNS TABLE(chronotype VARCHAR, peak_hour INT) AS $$
-- Implementation
$$ LANGUAGE plpgsql;
```

### Step 3: Topic Analysis Pipeline
Add to ML processor:

```
apps/ml-processor/src/stats/
├── __init__.py
├── yellow_stats.py      # Orchestrator
├── chronotype.py        # Time analysis
├── conversation.py      # Starter/replier analysis
├── response_time.py     # Speed analysis
├── influence.py         # Influence scoring
├── topics.py            # Embedding-based topic extraction
└── network.py           # Social graph analysis
```

### Step 4: API Endpoints
Extend existing card stats endpoint:

```
GET /api/v1/stats/users/{user_id}/card?chat_id={chat_id}&tier=yellow
```

### Step 5: Scheduled Jobs
- Topic clustering: Run after embedding batch (heavy computation)
- Other stats: Can run hourly or on-demand

---

## File Locations

| Component | Path |
|-----------|------|
| Migration | `apps/api-service/internal/migrations/sql/005_user_card_stats.sql` |
| Topic Analysis | `apps/ml-processor/src/stats/topics.py` |
| Network Analysis | `apps/ml-processor/src/stats/network.py` |
| Aggregation SQL | `apps/api-service/internal/repository/sql/yellow_stats.sql` |

---

## Output Example

```json
{
  "user_id": 123456789,
  "chat_id": -1003280306634,
  "yellow_stats": {
    "chronotype": {
      "type": "Night Owl",
      "peak_hour": 23,
      "distribution": {
        "morning": 12.5,
        "afternoon": 25.0,
        "evening": 35.0,
        "night": 27.5
      }
    },
    "conversation_style": {
      "type": "Initiator",
      "starter_pct": 72.3,
      "threads_started": 234,
      "replies_sent": 89
    },
    "response_speed": {
      "label": "Quick",
      "avg_seconds": 180,
      "median_seconds": 120
    },
    "influence": {
      "score": 78.5,
      "label": "Respected",
      "engagement_rate": 2.3,
      "unique_engagers": 45
    },
    "topics": [
      {"name": "Tech Discussion", "confidence": 0.45, "keywords": ["code", "api", "bug"]},
      {"name": "Food & Recipes", "confidence": 0.30, "keywords": ["recipe", "cook", "restaurant"]},
      {"name": "Gaming", "confidence": 0.15, "keywords": ["game", "play", "steam"]}
    ],
    "network": {
      "role": "Bridge",
      "reach_pct": 62.5,
      "unique_connections": 25
    }
  }
}
```

---

## Performance Considerations

| Stat | Complexity | Caching Strategy |
|------|------------|------------------|
| Chronotype | O(n) | Daily refresh |
| Conversation Style | O(n) | Daily refresh |
| Response Time | O(n log n) | Daily refresh |
| Influence | O(n²) joins | Hourly with incremental |
| Topics | O(n²) clustering | After embedding batches |
| Network | O(n²) | Daily refresh |

**Recommendations:**
- Run topic clustering as background job
- Cache influence scores with 1-hour TTL
- Use materialized views for network stats
