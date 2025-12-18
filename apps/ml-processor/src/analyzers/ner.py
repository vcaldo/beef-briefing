"""
Named Entity Recognition implementations.

Provides NER extraction for Portuguese text using spaCy
and API-based alternatives.
"""

import logging
from typing import Any

from src.analyzers.base import Analyzer, AnalysisType

logger = logging.getLogger(__name__)


class LocalNERExtractor(Analyzer):
    """
    NER extractor using spaCy.

    Default model: pt_core_news_lg
    - Languages: Portuguese
    - Entity types: PER, ORG, LOC, MISC
    - Quality: Trained on Portuguese news corpus

    To install the model:
        python -m spacy download pt_core_news_lg
    """

    def __init__(self, model_name: str = "pt_core_news_lg"):
        """
        Initialize the NER extractor.

        Args:
            model_name: spaCy model name (e.g., pt_core_news_lg)
        """
        self.model_name = model_name
        self._nlp = None  # Lazy loaded

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.NER

    def _load_model(self):
        """Lazy load the spaCy model on first use."""
        if self._nlp is None:
            import spacy

            try:
                self._nlp = spacy.load(self.model_name)
                logger.info(f"spaCy model loaded: {self.model_name}")
            except OSError:
                logger.error(
                    f"spaCy model '{self.model_name}' not found. "
                    f"Install with: python -m spacy download {self.model_name}"
                )
                raise

    def analyze(self, texts: list[str], **kwargs) -> list[list[dict]]:
        """
        Extract named entities from texts.

        Args:
            texts: List of texts to process

        Returns:
            List of entity lists (one list per text), where each entity is:
                - entity_type: str (e.g., 'PER', 'ORG', 'LOC', 'MISC')
                - entity_text: str
                - start_pos: int (character offset)
                - end_pos: int (character offset)
                - confidence: float (always 1.0 for spaCy)
        """
        if not texts:
            return []

        self._load_model()

        # Process in batches using spaCy's pipe
        results = []
        for doc in self._nlp.pipe(texts, batch_size=32):
            entities = []
            for ent in doc.ents:
                # Map spaCy entity labels to standard types
                entity_type = self._map_entity_type(ent.label_)
                entities.append(
                    {
                        "entity_type": entity_type,
                        "entity_text": ent.text,
                        "start_pos": ent.start_char,
                        "end_pos": ent.end_char,
                        "confidence": 1.0,  # spaCy doesn't provide confidence
                    }
                )
            results.append(entities)

        return results

    def _map_entity_type(self, label: str) -> str:
        """
        Map spaCy entity labels to standard types.

        spaCy Portuguese labels:
        - PER: Person
        - ORG: Organization
        - LOC: Location
        - MISC: Miscellaneous

        Args:
            label: spaCy entity label

        Returns:
            Standardized entity type
        """
        # spaCy Portuguese model uses these labels
        mapping = {
            "PER": "PERSON",
            "PESSOA": "PERSON",
            "ORG": "ORG",
            "ORGANIZACAO": "ORG",
            "LOC": "LOC",
            "LOCAL": "LOC",
            "MISC": "MISC",
            "OUTROS": "MISC",
            # English model labels (in case different model is used)
            "PERSON": "PERSON",
            "GPE": "LOC",  # Geopolitical entity -> Location
            "NORP": "ORG",  # Nationalities or religious/political groups
            "FAC": "LOC",  # Facility
            "EVENT": "MISC",
            "WORK_OF_ART": "MISC",
            "LAW": "MISC",
            "LANGUAGE": "MISC",
            "DATE": "DATE",
            "TIME": "TIME",
            "PERCENT": "NUMBER",
            "MONEY": "MONEY",
            "QUANTITY": "NUMBER",
            "ORDINAL": "NUMBER",
            "CARDINAL": "NUMBER",
        }
        return mapping.get(label, label)

    def is_available(self) -> bool:
        """Check if spaCy is available."""
        try:
            import spacy

            return True
        except ImportError:
            return False

    def cleanup(self) -> None:
        """Release model resources."""
        if self._nlp is not None:
            del self._nlp
            self._nlp = None
            logger.info("spaCy model unloaded")


class OpenAINERExtractor(Analyzer):
    """
    NER extractor using OpenAI API.

    Uses GPT-4o-mini for entity extraction via structured output.
    """

    def __init__(self, api_key: str | None = None):
        """
        Initialize with OpenAI API key.

        Args:
            api_key: OpenAI API key (required)
        """
        self.api_key = api_key
        self._client = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.NER

    def _get_client(self):
        """Lazy load the OpenAI client."""
        if self._client is None:
            from openai import OpenAI

            self._client = OpenAI(api_key=self.api_key)
        return self._client

    def analyze(self, texts: list[str], **kwargs) -> list[list[dict]]:
        """
        Extract named entities using OpenAI API.

        Args:
            texts: List of texts to process

        Returns:
            List of entity lists (one list per text)
        """
        if not texts:
            return []

        client = self._get_client()

        # Process texts in batches to reduce API calls
        prompt = """Extract named entities from each message. Return a JSON object with a "results" array.
Each result should be an array of entities found in that message.
Each entity should have:
- entity_type: "PERSON" | "ORG" | "LOC" | "DATE" | "MISC"
- entity_text: the actual text of the entity
- start_pos: null (not applicable for API extraction)
- end_pos: null
- confidence: estimated confidence 0-1

Messages:
"""
        for i, text in enumerate(texts):
            prompt += f"{i + 1}. {text[:500]}\n"

        try:
            response = client.chat.completions.create(
                model="gpt-4o-mini",
                messages=[{"role": "user", "content": prompt}],
                response_format={"type": "json_object"},
                temperature=0,
            )

            import json

            data = json.loads(response.choices[0].message.content)
            results = data.get("results", [])

            # Normalize results
            processed = []
            for i, entities in enumerate(results):
                if not isinstance(entities, list):
                    processed.append([])
                    continue

                normalized_entities = []
                for ent in entities:
                    normalized_entities.append(
                        {
                            "entity_type": ent.get("entity_type", "MISC"),
                            "entity_text": ent.get("entity_text", ""),
                            "start_pos": ent.get("start_pos"),
                            "end_pos": ent.get("end_pos"),
                            "confidence": ent.get("confidence", 0.8),
                        }
                    )
                processed.append(normalized_entities)

            # Pad if API returned fewer results
            while len(processed) < len(texts):
                processed.append([])

            return processed

        except Exception as e:
            logger.error(f"OpenAI NER extraction failed: {e}")
            # Return empty entities for all texts on failure
            return [[] for _ in texts]

    def is_available(self) -> bool:
        """Check if OpenAI API key is available."""
        return bool(self.api_key)

    def cleanup(self) -> None:
        """Release client resources."""
        self._client = None
