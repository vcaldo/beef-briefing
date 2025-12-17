"""
Dimensionality reduction for message embeddings using UMAP.
"""

import hashlib
import logging
import os
from pathlib import Path

import numpy as np

logger = logging.getLogger(__name__)


class EmbeddingReducer:
    """
    Reduce high-dimensional embeddings to 2D/3D for visualization.

    Uses UMAP with disk-based caching for performance.
    """

    def __init__(self, cache_dir: str = "/app/cache"):
        """
        Initialize the reducer with a cache directory.

        Args:
            cache_dir: Directory to store cached UMAP results
        """
        self.cache_dir = Path(cache_dir)
        self.cache_dir.mkdir(parents=True, exist_ok=True)

    def _get_cache_key(self, embeddings: np.ndarray, n_components: int) -> str:
        """Generate a cache key based on embedding content."""
        # Use shape and sample of data for faster hashing
        shape_bytes = np.array(embeddings.shape).tobytes()
        # Sample first, middle, and last embeddings for the hash
        sample_indices = [0, len(embeddings) // 2, -1]
        sample_bytes = b"".join(
            embeddings[i].tobytes() for i in sample_indices if i < len(embeddings)
        )
        combined = shape_bytes + sample_bytes + str(n_components).encode()
        return hashlib.md5(combined).hexdigest()

    def reduce(
        self,
        embeddings: np.ndarray,
        n_components: int = 2,
        n_neighbors: int = 15,
        min_dist: float = 0.1,
        use_cache: bool = True,
    ) -> np.ndarray:
        """
        Reduce embeddings to lower dimensions using UMAP.

        Args:
            embeddings: (N, 768) array of embeddings
            n_components: Target dimensions (2 or 3)
            n_neighbors: UMAP n_neighbors parameter
            min_dist: UMAP min_dist parameter
            use_cache: Whether to use disk caching

        Returns:
            (N, n_components) array of reduced coordinates
        """
        if len(embeddings) == 0:
            return np.array([])

        if len(embeddings) < 5:
            # Not enough points for UMAP, return random positions
            return np.random.rand(len(embeddings), n_components)

        # Check cache
        if use_cache:
            cache_key = self._get_cache_key(embeddings, n_components)
            cache_file = self.cache_dir / f"umap_{cache_key}.npy"

            if cache_file.exists():
                logger.info(f"Loading cached UMAP result from {cache_file}")
                try:
                    cached = np.load(cache_file)
                    if len(cached) == len(embeddings):
                        return cached
                except Exception as e:
                    logger.warning(f"Failed to load cache: {e}")

        # Import UMAP here to avoid slow import on startup
        import umap

        logger.info(
            f"Computing UMAP for {len(embeddings)} embeddings "
            f"(n_components={n_components}, n_neighbors={n_neighbors})"
        )

        # Adjust n_neighbors if we have fewer points
        effective_neighbors = min(n_neighbors, len(embeddings) - 1)

        reducer = umap.UMAP(
            n_components=n_components,
            n_neighbors=effective_neighbors,
            min_dist=min_dist,
            metric="cosine",
            random_state=42,
            low_memory=True,
        )

        reduced = reducer.fit_transform(embeddings)

        # Cache result
        if use_cache:
            try:
                np.save(cache_file, reduced)
                logger.info(f"Cached UMAP result to {cache_file}")
            except Exception as e:
                logger.warning(f"Failed to cache result: {e}")

        return reduced

    def clear_cache(self):
        """Clear all cached UMAP results."""
        for cache_file in self.cache_dir.glob("umap_*.npy"):
            try:
                cache_file.unlink()
            except Exception as e:
                logger.warning(f"Failed to delete {cache_file}: {e}")
        logger.info("Cleared UMAP cache")
