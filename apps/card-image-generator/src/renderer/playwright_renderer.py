"""Playwright-based HTML to PNG renderer."""

import logging

from playwright.async_api import async_playwright, Browser, Playwright

logger = logging.getLogger(__name__)


class PlaywrightRenderer:
    """Renders HTML content to PNG using Playwright headless browser."""

    def __init__(
        self,
        width: int = 400,
        height: int = 600,
        scale: int = 2,
    ):
        """
        Initialize renderer.

        Args:
            width: Card width in pixels
            height: Card height in pixels
            scale: Device scale factor (2 for retina)
        """
        self.width = width
        self.height = height
        self.scale = scale
        self._playwright: Playwright | None = None
        self._browser: Browser | None = None

    async def start(self) -> None:
        """Initialize Playwright and launch browser."""
        if self._browser:
            return

        logger.info("Starting Playwright browser...")
        self._playwright = await async_playwright().start()
        self._browser = await self._playwright.chromium.launch(
            headless=True,
            args=[
                "--no-sandbox",
                "--disable-gpu",
                "--disable-dev-shm-usage",
                "--disable-setuid-sandbox",
            ],
        )
        logger.info("Playwright browser started")

    async def stop(self) -> None:
        """Close browser and cleanup resources."""
        if self._browser:
            await self._browser.close()
            self._browser = None
        if self._playwright:
            await self._playwright.stop()
            self._playwright = None
        logger.info("Playwright browser stopped")

    async def render_html(
        self,
        html_content: str,
        base_url: str | None = None,
    ) -> bytes:
        """
        Render HTML content to PNG image.

        Args:
            html_content: Full HTML document to render
            base_url: Base URL for resolving relative paths (e.g., images, CSS)

        Returns:
            PNG image bytes
        """
        if not self._browser:
            await self.start()

        page = await self._browser.new_page(
            viewport={"width": self.width, "height": self.height},
            device_scale_factor=self.scale,
        )

        try:
            # Set content with base URL for relative paths
            await page.set_content(
                html_content,
                wait_until="networkidle",
            )

            # If base_url provided, use it for relative resources
            if base_url:
                await page.evaluate(f"""
                    const base = document.createElement('base');
                    base.href = '{base_url}';
                    document.head.prepend(base);
                """)

            # Wait for fonts and images to load
            await page.wait_for_load_state("networkidle")

            # Try to find .card element, otherwise screenshot full page
            card_element = await page.query_selector(".card")
            if card_element:
                screenshot = await card_element.screenshot(type="png", omit_background=True)
            else:
                screenshot = await page.screenshot(
                    type="png",
                    full_page=False,
                    omit_background=True,
                )

            return screenshot

        finally:
            await page.close()

    async def __aenter__(self):
        await self.start()
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.stop()
