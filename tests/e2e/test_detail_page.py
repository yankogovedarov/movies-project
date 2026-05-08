"""
E2E тестове за детайлния изглед на медия (/media/:id).
"""
import pytest
from playwright.sync_api import Page, expect

BASE_URL = "http://localhost:8080"


def _get_first_media_id(page: Page):
    """Връща ID-то на първата медия от ℹ бутона в списъка или None."""
    page.goto(BASE_URL)
    link = page.locator("a.icon-btn[title='Детайли']").first
    if link.count() == 0:
        return None
    href = link.get_attribute("href")
    if not href or "/media/" not in href:
        return None
    return href.split("/media/")[-1]


def test_detail_page_loads(page: Page):
    media_id = _get_first_media_id(page)
    if media_id is None:
        pytest.skip("No media available")
    page.goto(f"{BASE_URL}/media/{media_id}")
    expect(page.locator("h1")).to_be_visible()


def test_detail_has_back_link(page: Page):
    media_id = _get_first_media_id(page)
    if media_id is None:
        pytest.skip("No media available")
    page.goto(f"{BASE_URL}/media/{media_id}")
    expect(page.locator("a[href='/']")).to_be_visible()


def test_detail_has_tree_link(page: Page):
    media_id = _get_first_media_id(page)
    if media_id is None:
        pytest.skip("No media available")
    page.goto(f"{BASE_URL}/media/{media_id}")
    expect(page.locator("a[href='/tree']")).to_be_visible()


def test_detail_shows_history_sections(page: Page):
    media_id = _get_first_media_id(page)
    if media_id is None:
        pytest.skip("No media available")
    page.goto(f"{BASE_URL}/media/{media_id}")
    expect(page.locator("h2", has_text="История на стартиранията")).to_be_visible()
    expect(page.locator("h2", has_text="История на статусите")).to_be_visible()


def test_detail_has_status_dropdown(page: Page):
    media_id = _get_first_media_id(page)
    if media_id is None:
        pytest.skip("No media available")
    page.goto(f"{BASE_URL}/media/{media_id}")
    expect(page.locator("select[name=status]")).to_be_visible()


def test_detail_icon_button_opens_detail(page: Page):
    page.goto(BASE_URL)
    if page.locator("a.icon-btn[title='Детайли']").count() == 0:
        pytest.skip("No media available")
    page.locator("a.icon-btn[title='Детайли']").first.click()
    expect(page.locator("h1")).to_be_visible()
    assert "/media/" in page.url
