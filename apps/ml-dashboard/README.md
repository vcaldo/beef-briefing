# ML Analytics Dashboard

Interactive Streamlit dashboard for exploring ML-processed Telegram chat data. Visualizes sentiment analysis, toxicity detection, and message embeddings with UMAP projections.

## Tech Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| **Framework** | Streamlit 1.40+ | Web application framework |
| **Visualization** | Plotly 5.24+ | Interactive charts |
| **Database** | PostgreSQL + SQLAlchemy | ML results storage |
| **Vector DB** | Qdrant | Message embeddings (768-dim) |
| **Dimensionality Reduction** | UMAP | 768D → 2D/3D projection |
| **Caching** | Disk-based (numpy) | UMAP result caching |

## Features

### 1. Group Overview

Main dashboard showing aggregate group statistics.

**Visualizations:**
- **Metric Cards**: Messages analyzed, positive %, negative %, toxic %
- **User Behavior Quadrant**: Scatter plot of users in sentiment × toxicity space
- **Top Toxic Users**: Horizontal bar chart ranked by toxicity rate
- **Toxicity Timeline**: Daily toxic message count with rate overlay

**Quadrant Layout:**
```
           High Toxicity (100%)
                ↑
    ┌───────────┼───────────┐
    │  HOSTILE  │ PASSIONATE│
    │           │           │
Negative ←──────┼──────────→ Positive
    │           │           │
    │  CRITICAL │  FRIENDLY │
    └───────────┼───────────┘
                ↓
           Low Toxicity (0%)
```

### 2. User Analysis

Per-user drilldown with comparison to group averages.

**Visualizations:**
- **Comparison Metrics**: User sentiment/toxicity vs group average with delta
- **Highlighted Quadrant**: User position marked on group quadrant
- **Sentiment Timeline**: Stacked area chart (positive/neutral/negative per day)
- **Embedding Highlight**: User's messages highlighted in UMAP projection

### 3. Embedding Explorer

Interactive 2D/3D UMAP projection of message embeddings.

**Controls:**
- Dimensions: 2D or 3D projection
- Color by: Sentiment label or User
- Max points: 1,000 - 10,000 slider

**Visualizations:**
- Scatter plot with hover showing message preview
- Color-coded by sentiment (green/gray/red) or user highlight

## Database Queries

All queries use SQLAlchemy with raw SQL for performance. Located in `src/database/queries.py`.

### Chat Queries

| Method | Returns | Purpose |
|--------|---------|---------|
| `get_available_chats()` | `list[dict]` | Chats with ML data for sidebar |
| `get_chat_info(chat_id)` | `dict` | Basic chat metadata |

### Quadrant Queries

| Method | Returns | Purpose |
|--------|---------|---------|
| `get_user_behavior_quadrant(chat_id)` | `DataFrame` | User sentiment/toxicity for scatter |
| `get_user_details(user_id, chat_id)` | `dict` | Individual user profile |

**Quadrant Query:**
```sql
SELECT
    u.id as user_id,
    u.first_name,
    u.username,
    mup.avg_sentiment,      -- X-axis: -1 to +1
    mup.toxicity_rate,      -- Y-axis: 0 to 1
    mup.messages_analyzed   -- Dot size
FROM ml_user_profiles mup
JOIN users u ON u.id = mup.user_id
WHERE mup.chat_id = :chat_id
    AND mup.messages_analyzed >= 5
    AND u.is_bot = false
```

### Sentiment Queries

| Method | Returns | Purpose |
|--------|---------|---------|
| `get_sentiment_stats(chat_id)` | `dict` | Aggregate counts and rates |
| `get_sentiment_distribution(chat_id)` | `DataFrame` | Label breakdown for pie chart |

### Toxicity Queries

| Method | Returns | Purpose |
|--------|---------|---------|
| `get_toxicity_stats(chat_id)` | `dict` | Total analyzed, toxic count, rate |
| `get_toxicity_timeline(chat_id)` | `DataFrame` | Daily toxic counts for timeline |
| `get_user_toxicity_rankings(chat_id)` | `list[dict]` | Top toxic users |

**Timeline Query:**
```sql
SELECT
    DATE(m.date) as date,
    SUM(CASE WHEN mt.is_toxic THEN 1 ELSE 0 END) as toxic_count,
    COUNT(*) as total_count,
    ROUND(SUM(CASE WHEN mt.is_toxic THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2) as toxic_rate
FROM ml_toxicity mt
JOIN messages m ON m.id = mt.message_id
WHERE mt.chat_id = :chat_id
GROUP BY DATE(m.date)
ORDER BY date
```

### User Drilldown Queries

| Method | Returns | Purpose |
|--------|---------|---------|
| `get_user_sentiment_timeline(user_id, chat_id)` | `DataFrame` | Daily sentiment for user |
| `get_user_vs_group_comparison(user_id, chat_id)` | `dict` | User vs group averages + percentiles |

**Comparison Query:**
```sql
WITH user_stats AS (
    SELECT avg_sentiment, toxicity_rate
    FROM ml_user_profiles
    WHERE user_id = :user_id AND chat_id = :chat_id
),
group_stats AS (
    SELECT AVG(avg_sentiment), AVG(toxicity_rate)
    FROM ml_user_profiles
    WHERE chat_id = :chat_id
),
-- ... percentile calculations
SELECT
    user_avg_sentiment,
    group_avg_sentiment,
    user_toxicity_rate,
    group_toxicity_rate,
    sentiment_percentile,
    toxicity_percentile
```

### Embedding Queries

| Method | Returns | Purpose |
|--------|---------|---------|
| `get_messages_with_sentiment(chat_id)` | `DataFrame` | Messages + sentiment for coloring |
| `get_group_users(chat_id)` | `list[dict]` | Users for filtering |

## Vector Database (Qdrant)

Embeddings are fetched from Qdrant using `src/vector/qdrant_client.py`.

**Collection:** `message_embeddings`

**Schema:**
- **ID**: `message_id` (int)
- **Vector**: 768-dimensional float array
- **Payload**: `chat_id`, `user_id`, `text_preview`

**Fetch Method:**
```python
def get_embeddings_for_chat(self, chat_id: int, limit: int = 10000):
    # Scrolls through collection with chat_id filter
    # Returns: (embeddings: np.ndarray, metadata: list[dict])
```

## UMAP Reduction

Located in `src/embeddings/reducer.py`.

**Parameters:**
- `n_components`: 2 or 3
- `n_neighbors`: 15 (adjusted for small datasets)
- `min_dist`: 0.1
- `metric`: cosine

**Caching:**
- Results cached to disk as `.npy` files
- Cache key: MD5 hash of embedding content + dimensions
- Location: `/app/cache/umap_*.npy`

**Performance:**
- 5,000 embeddings: ~10-20 seconds
- 10,000 embeddings: ~30-60 seconds
- Cached results load in <1 second

## Configuration

Environment variables (see `config/__init__.py`):

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 5432 | PostgreSQL port |
| `DB_USER` | postgres | Database user |
| `DB_PASSWORD` | - | Database password |
| `DB_NAME` | beef_briefing | Database name |
| `QDRANT_HOST` | localhost | Qdrant host |
| `QDRANT_PORT` | 6333 | Qdrant port |
| `ML_DASHBOARD_PORT` | 8501 | Streamlit port |
| `CACHE_DIR` | /app/cache | UMAP cache directory |

## Running

### With Docker (recommended)

```bash
# Start all services including ml-dashboard
make up-build

# Or rebuild just ml-dashboard
make docker-build-ml-dashboard

# View logs
make docker-logs-ml-dashboard
```

Access at: http://localhost:8501

### Local Development

```bash
cd apps/ml-dashboard

# Create virtual environment
python -m venv venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=your_password
export DB_NAME=beef_briefing
export QDRANT_HOST=localhost
export QDRANT_PORT=6333

# Run
streamlit run app.py
```

## File Structure

```
apps/ml-dashboard/
├── Dockerfile
├── requirements.txt
├── README.md
├── app.py                     # Main Streamlit entrypoint
├── config/
│   └── __init__.py            # pydantic-settings config
└── src/
    ├── database/
    │   ├── connection.py      # SQLAlchemy engine
    │   └── queries.py         # All SQL queries
    ├── vector/
    │   └── qdrant_client.py   # Qdrant embedding fetch
    ├── embeddings/
    │   └── reducer.py         # UMAP with caching
    ├── charts/
    │   ├── quadrant.py        # User behavior scatter
    │   ├── clusters.py        # 2D/3D embedding viz
    │   └── toxicity.py        # Timeline + heatmap
    ├── components/
    │   ├── metrics.py         # Stat cards
    │   └── sidebar.py         # Navigation
    └── pages/
        ├── group_overview.py  # Main dashboard
        ├── user_drilldown.py  # Per-user view
        └── embedding_explorer.py
```

## Dependencies

Core dependencies from `requirements.txt`:

```
streamlit>=1.40.0      # Web framework
plotly>=5.24.0         # Charts
SQLAlchemy>=2.0.36     # Database
psycopg2-binary        # PostgreSQL driver
pandas>=2.2.3          # Data manipulation
qdrant-client>=1.12.0  # Vector database
umap-learn>=0.5.7      # Dimensionality reduction
pydantic-settings      # Configuration
```
