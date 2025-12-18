# User Cards: Red Stats Implementation Plan

**Tier:** 🔴 Advanced (New Models/Processing Required)
**Estimated Complexity:** High
**Dependencies:** New ML models, additional processing pipeline

---

## Overview

These stats require adding new ML models to the pipeline, fine-tuning existing models, or implementing complex NLP tasks. Each provides unique insights but comes with significant implementation cost.

---

## Stats to Implement

### 1. Humor Score (Comedy Detection)
**Challenge:** Detecting humor, jokes, and wit in Portuguese text
**Approach:** Multi-signal classification

**Signals to Combine:**
- 😂🤣 emoji presence and frequency
- "kkkk", "hahaha", "kkkkkkk" patterns (Brazilian laugh)
- Message context (replies with laugh reactions)
- Trained classifier on humor corpus

**Model Options:**

| Model | Pros | Cons |
|-------|------|------|
| Fine-tune BERT on humor dataset | Best accuracy | Requires labeled data |
| Zero-shot with LLM | No training needed | Expensive per-message |
| Heuristic + reaction correlation | Fast, simple | Lower accuracy |

**Recommended: Hybrid Approach**

```python
import re
from transformers import pipeline

class HumorDetector:
    def __init__(self):
        # Brazilian Portuguese laugh patterns
        self.laugh_patterns = [
            r'k{3,}',           # kkkk
            r'ha{2,}',          # hahaha
            r'rs{2,}',          # rsrsrs
            r'hua{2,}',         # huahua
            r'[😂🤣😆😹]{1,}',  # Laugh emojis
        ]

        # Optional: Load fine-tuned model if available
        self.classifier = None  # pipeline("text-classification", model="humor-pt-br")

    def detect_humor(self, text: str, reactions: list[str] = None) -> dict:
        score = 0.0
        signals = []

        # Signal 1: Laugh patterns in text
        for pattern in self.laugh_patterns:
            if re.search(pattern, text.lower()):
                score += 0.3
                signals.append("laugh_pattern")
                break

        # Signal 2: Laugh emojis in message
        laugh_emojis = len(re.findall(r'[😂🤣😆😹]', text))
        if laugh_emojis > 0:
            score += min(0.2, laugh_emojis * 0.1)
            signals.append("laugh_emoji")

        # Signal 3: Reactions received (if available)
        if reactions:
            laugh_reactions = sum(1 for r in reactions if r in ['😂', '🤣', '😆'])
            if laugh_reactions > 0:
                score += min(0.4, laugh_reactions * 0.1)
                signals.append("laugh_reactions")

        # Signal 4: Model prediction (if available)
        if self.classifier and len(text) > 10:
            pred = self.classifier(text)[0]
            if pred['label'] == 'humor':
                score += pred['score'] * 0.3
                signals.append("model")

        return {
            "is_funny": score > 0.4,
            "humor_score": min(1.0, score),
            "signals": signals
        }

    def user_humor_score(self, messages: list[dict]) -> dict:
        """Aggregate humor score for a user"""
        scores = [self.detect_humor(m['text'], m.get('reactions'))['humor_score']
                  for m in messages if m.get('text')]

        if not scores:
            return {"score": 0, "label": "Unknown"}

        avg_score = sum(scores) / len(scores)
        funny_count = sum(1 for s in scores if s > 0.4)

        return {
            "score": round(avg_score * 100, 1),
            "funny_messages": funny_count,
            "funny_rate": round(funny_count / len(scores) * 100, 1),
            "label": self._get_label(avg_score)
        }

    def _get_label(self, score: float) -> str:
        if score > 0.6: return "Comedian"
        if score > 0.4: return "Witty"
        if score > 0.2: return "Occasional Joker"
        return "Serious"
```

**Labels:**
- "Comedian" 🎭 (>60% humor score) - Natural entertainer
- "Witty" 😏 (40-60%) - Good sense of humor
- "Occasional Joker" 🙂 (20-40%) - Sometimes funny
- "Serious" 📊 (<20%) - Business-focused

---

### 2. Question Master vs Oracle
**Challenge:** Classify messages as questions vs answers/statements
**Approach:** Text classification model

**Model Options:**

| Approach | Implementation |
|----------|---------------|
| Rule-based | Check for `?`, question words |
| Fine-tuned BERT | Best accuracy, needs training data |
| Zero-shot classification | Use existing NLI model |

**Recommended: Zero-Shot Classification**

```python
from transformers import pipeline

class QuestionClassifier:
    def __init__(self):
        self.classifier = pipeline(
            "zero-shot-classification",
            model="facebook/bart-large-mnli"  # or multilingual variant
        )
        self.labels = ["question", "answer", "statement", "opinion"]

    def classify(self, text: str) -> dict:
        # Quick heuristic check first
        if text.strip().endswith('?'):
            return {"type": "question", "confidence": 0.95}

        # Question words in Portuguese
        question_starters = ['quem', 'qual', 'quando', 'onde', 'como', 'por que', 'porque', 'o que', 'quanto']
        text_lower = text.lower()
        if any(text_lower.startswith(q) for q in question_starters):
            return {"type": "question", "confidence": 0.9}

        # Use model for ambiguous cases
        result = self.classifier(text, self.labels)
        return {
            "type": result['labels'][0],
            "confidence": result['scores'][0]
        }

    def user_style(self, messages: list[str]) -> dict:
        classifications = [self.classify(m) for m in messages if m]

        type_counts = {}
        for c in classifications:
            t = c['type']
            type_counts[t] = type_counts.get(t, 0) + 1

        total = len(classifications)
        question_pct = type_counts.get('question', 0) / total * 100 if total else 0

        return {
            "question_pct": round(question_pct, 1),
            "answer_pct": round(type_counts.get('answer', 0) / total * 100, 1) if total else 0,
            "label": self._get_label(question_pct),
            "breakdown": type_counts
        }

    def _get_label(self, question_pct: float) -> str:
        if question_pct > 50: return "Curious Mind"
        if question_pct > 30: return "Inquisitive"
        if question_pct > 15: return "Balanced"
        return "Oracle"
```

**Labels:**
- "Curious Mind" ❓ (>50% questions) - Always asking
- "Inquisitive" 🤔 (30-50%) - Asks thoughtful questions
- "Balanced" ⚖️ (15-30%) - Mix of questions and answers
- "Oracle" 🔮 (<15%) - Mostly provides answers

---

### 3. Link Curator Score
**Challenge:** Track external content sharing behavior
**Approach:** URL extraction + domain categorization

```python
import re
from urllib.parse import urlparse
from collections import Counter

class LinkCurator:
    def __init__(self):
        self.url_pattern = re.compile(
            r'https?://[^\s<>"{}|\\^`\[\]]+'
        )

        # Domain categories
        self.categories = {
            'news': ['g1.com', 'uol.com', 'folha.uol', 'estadao.com', 'bbc.com'],
            'social': ['twitter.com', 'x.com', 'instagram.com', 'facebook.com', 'tiktok.com'],
            'video': ['youtube.com', 'youtu.be', 'vimeo.com', 'twitch.tv'],
            'tech': ['github.com', 'stackoverflow.com', 'medium.com', 'dev.to'],
            'shopping': ['amazon.com', 'mercadolivre.com', 'aliexpress.com'],
            'music': ['spotify.com', 'soundcloud.com', 'deezer.com'],
        }

    def extract_links(self, text: str) -> list[dict]:
        urls = self.url_pattern.findall(text)
        results = []
        for url in urls:
            try:
                parsed = urlparse(url)
                domain = parsed.netloc.lower().replace('www.', '')
                category = self._categorize_domain(domain)
                results.append({
                    "url": url,
                    "domain": domain,
                    "category": category
                })
            except:
                continue
        return results

    def _categorize_domain(self, domain: str) -> str:
        for category, domains in self.categories.items():
            if any(d in domain for d in domains):
                return category
        return "other"

    def user_curator_score(self, messages: list[str]) -> dict:
        all_links = []
        messages_with_links = 0

        for msg in messages:
            links = self.extract_links(msg)
            if links:
                messages_with_links += 1
                all_links.extend(links)

        if not all_links:
            return {"score": 0, "label": "Non-Sharer", "categories": {}}

        # Analyze categories
        categories = Counter(l['category'] for l in all_links)
        domains = Counter(l['domain'] for l in all_links)

        # Diversity score (unique domains / total links)
        diversity = len(domains) / len(all_links)

        # Sharing rate
        share_rate = messages_with_links / len(messages) if messages else 0

        return {
            "total_links": len(all_links),
            "unique_domains": len(domains),
            "share_rate": round(share_rate * 100, 1),
            "diversity_score": round(diversity * 100, 1),
            "top_categories": dict(categories.most_common(3)),
            "top_domains": dict(domains.most_common(5)),
            "label": self._get_label(share_rate, diversity)
        }

    def _get_label(self, share_rate: float, diversity: float) -> str:
        if share_rate > 0.1 and diversity > 0.5:
            return "Content Curator"
        if share_rate > 0.1:
            return "Link Bomber"
        if share_rate > 0.05:
            return "Occasional Sharer"
        return "Non-Sharer"
```

**Labels:**
- "Content Curator" 📚 (>10% share rate, diverse) - Quality link sharing
- "Link Bomber" 💣 (>10% share rate, repetitive) - Lots of links
- "Occasional Sharer" 📎 (5-10%) - Sometimes shares
- "Non-Sharer" 🚫 (<5%) - Rarely shares links

---

### 4. Sarcasm Detection
**Challenge:** Detecting sarcasm/irony in Portuguese text
**Approach:** Fine-tuned model + contextual analysis

**This is one of the hardest NLP tasks. Options:**

| Approach | Accuracy | Cost |
|----------|----------|------|
| LLM zero-shot | ~70% | High per-message |
| Fine-tuned BERT | ~75% | Medium (training) |
| Heuristic signals | ~50% | Low |

**Recommended: Signal-based + Optional LLM**

```python
import re

class SarcasmDetector:
    def __init__(self, use_llm: bool = False):
        self.use_llm = use_llm

        # Sarcasm indicators
        self.indicators = {
            'caps_ratio': 0.5,      # MaS qUe LiNdO
            'ellipsis': r'\.{3,}',  # Que legal...
            'quotes': r'"[^"]+"|\'[^\']+\'',  # "Adorei"
            'emoji_contrast': ['🙃', '😒', '🤷', '😏'],
            'obvious_phrases': [
                'que surpresa', 'nossa que legal', 'imagina',
                'claro que sim', 'com certeza', 'óbvio',
                'parabéns', 'incrível'  # context-dependent
            ]
        }

    def detect(self, text: str, context: dict = None) -> dict:
        score = 0.0
        signals = []

        # Signal 1: Mixed case (sarcastic emphasis)
        if text != text.lower() and text != text.upper():
            upper_ratio = sum(1 for c in text if c.isupper()) / max(len(text), 1)
            if 0.3 < upper_ratio < 0.7:
                score += 0.2
                signals.append("mixed_case")

        # Signal 2: Excessive punctuation
        if re.search(r'[!?]{2,}', text) or re.search(self.indicators['ellipsis'], text):
            score += 0.15
            signals.append("punctuation")

        # Signal 3: Quoted words (air quotes)
        if re.search(self.indicators['quotes'], text):
            score += 0.2
            signals.append("quotes")

        # Signal 4: Sarcasm emojis
        if any(e in text for e in self.indicators['emoji_contrast']):
            score += 0.25
            signals.append("emoji")

        # Signal 5: Known sarcastic phrases
        text_lower = text.lower()
        for phrase in self.indicators['obvious_phrases']:
            if phrase in text_lower:
                score += 0.15
                signals.append(f"phrase:{phrase}")
                break

        # Signal 6: Context (if sentiment contradicts reactions)
        if context and context.get('sentiment') == 'positive' and context.get('reactions_negative'):
            score += 0.3
            signals.append("context_mismatch")

        return {
            "is_sarcastic": score > 0.4,
            "sarcasm_score": min(1.0, score),
            "signals": signals,
            "confidence": "low" if len(signals) < 2 else "medium" if len(signals) < 4 else "high"
        }

    def user_sarcasm_level(self, messages: list[dict]) -> dict:
        results = [self.detect(m['text'], m.get('context')) for m in messages if m.get('text')]

        if not results:
            return {"score": 0, "label": "Unknown"}

        avg_score = sum(r['sarcasm_score'] for r in results) / len(results)
        sarcastic_count = sum(1 for r in results if r['is_sarcastic'])

        return {
            "score": round(avg_score * 100, 1),
            "sarcastic_messages": sarcastic_count,
            "sarcasm_rate": round(sarcastic_count / len(results) * 100, 1),
            "label": self._get_label(avg_score)
        }

    def _get_label(self, score: float) -> str:
        if score > 0.5: return "Master of Irony"
        if score > 0.3: return "Sarcastic"
        if score > 0.15: return "Occasional Wit"
        return "Straightforward"
```

**Labels:**
- "Master of Irony" 🎭 (>50%) - Highly sarcastic
- "Sarcastic" 😏 (30-50%) - Often sarcastic
- "Occasional Wit" 🙃 (15-30%) - Sometimes sarcastic
- "Straightforward" 📢 (<15%) - Direct communicator

---

### 5. Named Entity Recognition (Interests)
**Challenge:** Extract people, places, organizations, and topics mentioned
**Approach:** NER model for Portuguese

**Model:** `neuralmind/bert-base-portuguese-cased` + NER head, or spaCy Portuguese

```python
import spacy
from collections import Counter

class InterestExtractor:
    def __init__(self):
        # Load Portuguese NER model
        self.nlp = spacy.load("pt_core_news_lg")

    def extract_entities(self, text: str) -> list[dict]:
        doc = self.nlp(text)
        entities = []
        for ent in doc.ents:
            entities.append({
                "text": ent.text,
                "label": ent.label_,
                "start": ent.start_char,
                "end": ent.end_char
            })
        return entities

    def user_interests(self, messages: list[str]) -> dict:
        all_entities = []
        for msg in messages:
            entities = self.extract_entities(msg)
            all_entities.extend(entities)

        # Group by type
        by_type = {}
        for ent in all_entities:
            label = ent['label']
            if label not in by_type:
                by_type[label] = []
            by_type[label].append(ent['text'].lower())

        # Get top entities per type
        interests = {}
        for label, entities in by_type.items():
            counter = Counter(entities)
            interests[label] = [
                {"name": name, "count": count}
                for name, count in counter.most_common(5)
            ]

        # Derive interest profile
        return {
            "entities": interests,
            "top_people": interests.get('PER', [])[:3],
            "top_places": interests.get('LOC', [])[:3],
            "top_orgs": interests.get('ORG', [])[:3],
            "interest_diversity": len(by_type),
            "label": self._get_label(interests)
        }

    def _get_label(self, interests: dict) -> str:
        total = sum(len(v) for v in interests.values())
        if interests.get('PER') and len(interests['PER']) > total * 0.4:
            return "People Person"
        if interests.get('LOC') and len(interests['LOC']) > total * 0.3:
            return "Globe Trotter"
        if interests.get('ORG') and len(interests['ORG']) > total * 0.3:
            return "Industry Watcher"
        return "Diverse Interests"
```

**Entity Types (spaCy Portuguese):**
- `PER` - People
- `LOC` - Locations
- `ORG` - Organizations
- `MISC` - Miscellaneous

**Labels:**
- "People Person" 👥 - Talks about people often
- "Globe Trotter" 🌍 - Mentions many places
- "Industry Watcher" 🏢 - Discusses organizations
- "Diverse Interests" 🎯 - Varied topics

---

## Database Schema Additions

```sql
-- Add to ml_user_card_stats table
ALTER TABLE ml_user_card_stats ADD COLUMN IF NOT EXISTS
    -- Red stats
    humor_score NUMERIC(5,2),
    humor_label VARCHAR(30),
    funny_message_count INTEGER,

    question_pct NUMERIC(5,2),
    question_style VARCHAR(30),

    link_share_rate NUMERIC(5,2),
    link_curator_label VARCHAR(30),
    top_link_categories JSONB,

    sarcasm_score NUMERIC(5,2),
    sarcasm_label VARCHAR(30),

    named_interests JSONB,  -- {people: [], places: [], orgs: []}
    interest_label VARCHAR(30);

-- Detailed entity storage (optional)
CREATE TABLE IF NOT EXISTS ml_user_entities (
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    entity_type VARCHAR(10) NOT NULL,  -- PER, LOC, ORG, MISC
    entity_text VARCHAR(255) NOT NULL,
    mention_count INTEGER DEFAULT 1,
    first_seen TIMESTAMPTZ DEFAULT NOW(),
    last_seen TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, chat_id, entity_type, entity_text)
);

CREATE INDEX idx_user_entities_chat ON ml_user_entities(chat_id);
CREATE INDEX idx_user_entities_type ON ml_user_entities(entity_type);
```

---

## Implementation Architecture

### New Models to Add

```
apps/ml-processor/src/models/
├── sentiment.py      # Existing
├── toxicity.py       # Existing
├── embeddings.py     # Existing
├── humor.py          # NEW: Humor detection
├── questions.py      # NEW: Question classification
├── sarcasm.py        # NEW: Sarcasm detection
└── ner.py            # NEW: Named entity recognition
```

### Pipeline Extension

```python
# apps/ml-processor/src/pipeline/red_processor.py

class RedStatsProcessor:
    def __init__(self):
        self.humor_detector = HumorDetector()
        self.question_classifier = QuestionClassifier()
        self.link_curator = LinkCurator()
        self.sarcasm_detector = SarcasmDetector()
        self.ner_extractor = InterestExtractor()

    async def process_user(self, user_id: int, chat_id: int, messages: list[dict]) -> dict:
        return {
            "humor": self.humor_detector.user_humor_score(messages),
            "questions": self.question_classifier.user_style([m['text'] for m in messages]),
            "links": self.link_curator.user_curator_score([m['text'] for m in messages]),
            "sarcasm": self.sarcasm_detector.user_sarcasm_level(messages),
            "interests": self.ner_extractor.user_interests([m['text'] for m in messages])
        }
```

---

## Model Requirements

| Model | Size | GPU Memory | Processing Time |
|-------|------|------------|-----------------|
| Humor (BERT fine-tuned) | ~440MB | ~2GB | ~10ms/message |
| Question (BART-MNLI) | ~1.6GB | ~4GB | ~50ms/message |
| Sarcasm (heuristic) | N/A | N/A | <1ms/message |
| NER (spaCy lg) | ~550MB | ~1GB | ~5ms/message |

**Total Additional GPU Memory:** ~7GB (can be reduced with model sharing)

---

## Implementation Steps

### Phase 1: Infrastructure
1. Add new model files to ml-processor
2. Update requirements.txt with new dependencies:
   ```
   spacy>=3.7.0
   pt-core-news-lg @ https://github.com/explosion/spacy-models/releases/download/pt_core_news_lg-3.7.0/pt_core_news_lg-3.7.0.tar.gz
   ```
3. Extend database schema

### Phase 2: Individual Models
1. Implement and test HumorDetector
2. Implement and test QuestionClassifier
3. Implement and test LinkCurator
4. Implement and test SarcasmDetector
5. Implement and test InterestExtractor

### Phase 3: Integration
1. Create RedStatsProcessor orchestrator
2. Add API endpoint for red stats
3. Integrate into existing card stats response

### Phase 4: Optimization
1. Batch processing for efficiency
2. Caching for repeated user queries
3. Incremental updates (new messages only)

---

## API Response Example

```json
{
  "user_id": 123456789,
  "chat_id": -1003280306634,
  "red_stats": {
    "humor": {
      "score": 45.2,
      "label": "Witty",
      "funny_messages": 89,
      "funny_rate": 12.3
    },
    "question_style": {
      "type": "Inquisitive",
      "question_pct": 35.2,
      "answer_pct": 28.1,
      "breakdown": {"question": 256, "answer": 204, "statement": 265, "opinion": 0}
    },
    "link_curator": {
      "label": "Content Curator",
      "share_rate": 8.5,
      "diversity_score": 72.3,
      "top_categories": {"tech": 45, "video": 23, "news": 12}
    },
    "sarcasm": {
      "score": 22.1,
      "label": "Occasional Wit",
      "sarcasm_rate": 8.5
    },
    "interests": {
      "label": "Industry Watcher",
      "top_people": [{"name": "elon musk", "count": 12}],
      "top_places": [{"name": "são paulo", "count": 8}],
      "top_orgs": [{"name": "google", "count": 15}, {"name": "openai", "count": 10}]
    }
  }
}
```

---

## Cost-Benefit Analysis

| Stat | Implementation Cost | User Value | Recommendation |
|------|---------------------|------------|----------------|
| Humor Score | Medium | High | Implement |
| Question Style | Low | Medium | Implement |
| Link Curator | Low | Medium | Implement |
| Sarcasm | High | Medium | Optional |
| NER Interests | Medium | High | Implement |

**Recommended Priority:**
1. Link Curator (easy, no new models)
2. Question Style (reuse existing zero-shot)
3. Humor Score (high user value)
4. NER Interests (rich data)
5. Sarcasm (last, hardest)

---

## Alternative: LLM-Based Approach

Instead of multiple specialized models, use a single LLM call per user batch:

```python
async def analyze_user_with_llm(messages: list[str]) -> dict:
    prompt = f"""Analyze these messages from a user and provide:
    1. Humor score (0-100) and label
    2. Question vs answer tendency
    3. Sarcasm level
    4. Key interests/topics mentioned

    Messages:
    {messages[:50]}  # Sample

    Return JSON with: humor, question_style, sarcasm, interests
    """

    response = await llm.generate(prompt)
    return parse_json(response)
```

**Pros:** Single model, consistent results, handles edge cases better
**Cons:** Higher cost per user, rate limits, latency
