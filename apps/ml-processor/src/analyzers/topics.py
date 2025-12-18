"""
Topic clustering implementations.

Provides clustering of messages into topics using embeddings
and keyword extraction.
"""

import logging
from collections import Counter
from typing import Any

import numpy as np

from src.analyzers.base import Analyzer, AnalysisType

logger = logging.getLogger(__name__)


class LocalTopicClusterer(Analyzer):
    """
    Topic clusterer using HDBSCAN on embeddings.

    Uses HDBSCAN for density-based clustering and TF-IDF for
    keyword extraction per cluster.

    Note: This analyzer requires embeddings to be passed in,
    unlike other analyzers which work directly on text.
    """

    def __init__(
        self,
        min_cluster_size: int = 5,
        min_samples: int = 3,
        cluster_selection_epsilon: float = 0.0,
    ):
        """
        Initialize the topic clusterer.

        Args:
            min_cluster_size: Minimum cluster size for HDBSCAN
            min_samples: Minimum samples for core point (HDBSCAN)
            cluster_selection_epsilon: Distance threshold for cluster merging
        """
        self.min_cluster_size = min_cluster_size
        self.min_samples = min_samples
        self.cluster_selection_epsilon = cluster_selection_epsilon

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOPICS

    def analyze(
        self,
        texts: list[str],
        embeddings: np.ndarray | None = None,
        **kwargs,
    ) -> list[dict]:
        """
        Cluster texts into topics.

        Args:
            texts: List of texts (used for keyword extraction)
            embeddings: Pre-computed embeddings array (n_texts, embedding_dim)
                        If None, returns empty results

        Returns:
            List of dicts with:
                - topic_id: int (-1 for outliers)
                - similarity: float (distance to cluster centroid)
                - is_outlier: bool
        """
        if not texts or embeddings is None or len(embeddings) == 0:
            return []

        if len(texts) != len(embeddings):
            logger.warning(
                f"Text count ({len(texts)}) != embedding count ({len(embeddings)})"
            )
            return []

        # Run HDBSCAN clustering
        from sklearn.cluster import HDBSCAN

        clusterer = HDBSCAN(
            min_cluster_size=self.min_cluster_size,
            min_samples=self.min_samples,
            cluster_selection_epsilon=self.cluster_selection_epsilon,
            metric="euclidean",
        )

        labels = clusterer.fit_predict(embeddings)

        # Calculate similarity scores (inverse of distance to centroid)
        similarities = self._compute_similarities(embeddings, labels)

        results = []
        for i, label in enumerate(labels):
            results.append(
                {
                    "topic_id": int(label),
                    "similarity": float(similarities[i]),
                    "is_outlier": label == -1,
                }
            )

        # Log cluster statistics
        unique_labels = set(labels)
        n_clusters = len([l for l in unique_labels if l >= 0])
        n_outliers = sum(1 for l in labels if l == -1)
        logger.info(
            f"Topic clustering: {n_clusters} clusters, {n_outliers} outliers "
            f"from {len(texts)} texts"
        )

        return results

    def _compute_similarities(
        self, embeddings: np.ndarray, labels: np.ndarray
    ) -> np.ndarray:
        """
        Compute similarity scores (1 - normalized distance to centroid).

        Args:
            embeddings: Embedding vectors
            labels: Cluster labels from HDBSCAN

        Returns:
            Array of similarity scores (0-1, higher = closer to centroid)
        """
        similarities = np.zeros(len(labels))

        # Compute centroids for each cluster
        unique_labels = set(labels)
        centroids = {}
        max_distances = {}

        for label in unique_labels:
            if label == -1:
                continue  # Skip outliers
            mask = labels == label
            cluster_embeddings = embeddings[mask]
            centroids[label] = cluster_embeddings.mean(axis=0)

            # Compute max distance for normalization
            distances = np.linalg.norm(
                cluster_embeddings - centroids[label], axis=1
            )
            max_distances[label] = max(distances.max(), 1e-6)

        # Compute normalized similarity for each point
        for i, label in enumerate(labels):
            if label == -1:
                similarities[i] = 0.0
            else:
                distance = np.linalg.norm(embeddings[i] - centroids[label])
                # Normalize and invert (closer = higher similarity)
                similarities[i] = 1 - (distance / max_distances[label])

        return similarities

    def extract_keywords(
        self,
        texts: list[str],
        labels: list[int],
        top_k: int = 5,
    ) -> dict[int, list[str]]:
        """
        Extract top keywords per cluster using TF-IDF.

        Args:
            texts: List of texts
            labels: Cluster labels for each text
            top_k: Number of keywords to extract per cluster

        Returns:
            Dict mapping topic_id to list of keywords
        """
        from sklearn.feature_extraction.text import TfidfVectorizer

        cluster_keywords = {}
        unique_labels = set(labels)

        for label in unique_labels:
            if label == -1:
                continue  # Skip outliers

            # Get texts for this cluster
            cluster_texts = [texts[i] for i, l in enumerate(labels) if l == label]

            if not cluster_texts:
                continue

            # Fit TF-IDF on cluster texts
            try:
                vectorizer = TfidfVectorizer(
                    max_features=100,
                    stop_words=None,  # Portuguese stop words handled separately
                    min_df=1,
                    max_df=0.95,
                )
                tfidf_matrix = vectorizer.fit_transform(cluster_texts)

                # Get feature names
                feature_names = vectorizer.get_feature_names_out()

                # Sum TF-IDF scores across documents
                scores = tfidf_matrix.sum(axis=0).A1

                # Get top keywords
                top_indices = scores.argsort()[-top_k:][::-1]
                keywords = [feature_names[i] for i in top_indices]

                cluster_keywords[label] = keywords

            except Exception as e:
                logger.warning(f"Keyword extraction failed for cluster {label}: {e}")
                cluster_keywords[label] = []

        return cluster_keywords

    def get_cluster_stats(
        self, labels: list[int]
    ) -> dict[int, int]:
        """
        Get message count per cluster.

        Args:
            labels: Cluster labels

        Returns:
            Dict mapping topic_id to message count
        """
        counts = Counter(labels)
        # Remove outliers from stats
        counts.pop(-1, None)
        return dict(counts)

    def is_available(self) -> bool:
        """Check if sklearn is available."""
        try:
            from sklearn.cluster import HDBSCAN

            return True
        except ImportError:
            return False

    def cleanup(self) -> None:
        """No resources to release."""
        pass


class OpenAITopicClusterer(Analyzer):
    """
    Topic clusterer using OpenAI embeddings + local HDBSCAN.

    Uses OpenAI's text-embedding-3-small for embeddings,
    then clusters locally with HDBSCAN.
    """

    def __init__(
        self,
        api_key: str | None = None,
        min_cluster_size: int = 5,
        min_samples: int = 3,
    ):
        """
        Initialize with OpenAI API key.

        Args:
            api_key: OpenAI API key (required)
            min_cluster_size: Minimum cluster size for HDBSCAN
            min_samples: Minimum samples for core point
        """
        self.api_key = api_key
        self.min_cluster_size = min_cluster_size
        self.min_samples = min_samples
        self._client = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOPICS

    def _get_client(self):
        """Lazy load the OpenAI client."""
        if self._client is None:
            from openai import OpenAI

            self._client = OpenAI(api_key=self.api_key)
        return self._client

    def analyze(
        self,
        texts: list[str],
        embeddings: np.ndarray | None = None,
        **kwargs,
    ) -> list[dict]:
        """
        Cluster texts using OpenAI embeddings.

        Args:
            texts: List of texts to cluster
            embeddings: Ignored - embeddings are fetched from OpenAI

        Returns:
            List of topic assignment dicts
        """
        if not texts:
            return []

        client = self._get_client()

        # Get embeddings from OpenAI
        response = client.embeddings.create(
            model="text-embedding-3-small",
            input=texts,
        )

        embeddings = np.array([e.embedding for e in response.data])

        # Use local clusterer for the actual clustering
        local_clusterer = LocalTopicClusterer(
            min_cluster_size=self.min_cluster_size,
            min_samples=self.min_samples,
        )

        return local_clusterer.analyze(texts, embeddings=embeddings)

    def is_available(self) -> bool:
        """Check if OpenAI API key is available."""
        return bool(self.api_key)

    def cleanup(self) -> None:
        """Release client resources."""
        self._client = None
