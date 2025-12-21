"""MinIO client for card image storage."""

import hashlib
import logging
from datetime import timedelta
from io import BytesIO

from minio import Minio
from minio.error import S3Error

logger = logging.getLogger(__name__)


class CardStorageClient:
    """MinIO client for uploading and retrieving card images."""

    def __init__(
        self,
        endpoint: str,
        access_key: str,
        secret_key: str,
        bucket: str,
        use_ssl: bool = False,
        region: str = "",
    ):
        self.client = Minio(
            endpoint,
            access_key=access_key,
            secret_key=secret_key,
            secure=use_ssl,
            region=region or None,
        )
        self.bucket = bucket
        self._ensure_bucket()

    def _ensure_bucket(self) -> None:
        """Create bucket if it doesn't exist."""
        try:
            if not self.client.bucket_exists(self.bucket):
                self.client.make_bucket(self.bucket)
                logger.info(f"Created bucket: {self.bucket}")
        except S3Error as e:
            logger.error(f"Failed to ensure bucket exists: {e}")
            raise

    @staticmethod
    def compute_hash(data: bytes) -> str:
        """Compute SHA256 hash of data."""
        return hashlib.sha256(data).hexdigest()

    @staticmethod
    def generate_storage_path(
        chat_id: int,
        week_start: str,
        user_id: int,
        theme: str = "gaming",
    ) -> str:
        """
        Generate storage path for a card image.

        Format: cards/{chat_id}/{week_start}/{theme}/{user_id}.png
        """
        return f"cards/{chat_id}/{week_start}/{theme}/{user_id}.png"

    def upload_card_image(
        self,
        chat_id: int,
        week_start: str,
        user_id: int,
        image_data: bytes,
        theme: str = "gaming",
    ) -> tuple[str, str, int]:
        """
        Upload card image to MinIO.

        Args:
            chat_id: Chat ID
            week_start: Week start date (YYYY-MM-DD)
            user_id: User ID
            image_data: PNG image bytes
            theme: Theme name for storage path

        Returns:
            Tuple of (storage_path, file_hash, file_size)
        """
        file_hash = self.compute_hash(image_data)
        storage_path = self.generate_storage_path(chat_id, week_start, user_id, theme)
        file_size = len(image_data)

        # Check if identical file already exists
        try:
            stat = self.client.stat_object(self.bucket, storage_path)
            existing_hash = stat.metadata.get("x-amz-meta-file-hash")
            if existing_hash == file_hash:
                logger.debug(f"Identical file exists, skipping upload: {storage_path}")
                return storage_path, file_hash, file_size
        except S3Error as e:
            # Only log unexpected errors (not "object doesn't exist")
            if e.code != "NoSuchKey":
                logger.debug(f"S3 stat check failed for {storage_path}: {e.code}")

        # Upload with metadata
        self.client.put_object(
            self.bucket,
            storage_path,
            BytesIO(image_data),
            length=file_size,
            content_type="image/png",
            metadata={"file-hash": file_hash},
        )

        logger.info(f"Uploaded card image: {storage_path} ({file_size} bytes)")
        return storage_path, file_hash, file_size

    def get_presigned_url(
        self,
        storage_path: str,
        expires_seconds: int = 3600,
    ) -> str:
        """
        Generate a presigned URL for accessing an image.

        Args:
            storage_path: Object key in bucket
            expires_seconds: URL expiration time in seconds

        Returns:
            Presigned URL string
        """
        return self.client.presigned_get_object(
            self.bucket,
            storage_path,
            expires=timedelta(seconds=expires_seconds),
        )

    def delete_card_image(self, storage_path: str) -> bool:
        """
        Delete a card image from storage.

        Returns True if deleted, False if not found.
        """
        try:
            self.client.remove_object(self.bucket, storage_path)
            logger.info(f"Deleted card image: {storage_path}")
            return True
        except S3Error as e:
            if e.code == "NoSuchKey":
                return False
            raise

    def image_exists(self, storage_path: str) -> bool:
        """Check if an image exists in storage."""
        try:
            self.client.stat_object(self.bucket, storage_path)
            return True
        except S3Error:
            return False
