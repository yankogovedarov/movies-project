"""
E2E тестове за взаимодействие с flat list изгледа.
Тестовете, изискващи медии, се пропускат ако данните не са налични.
"""
import pytest
from playwright.sync_api import Page, expect

BASE_URL = "http://localhost:8080"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _has_media(page: Page) -> bool:
    return page.locator("td.filename button.filename-link").count() > 0


# ---------------------------------------------------------------------------
# Layout / структура
# ---------------------------------------------------------------------------

def test_list_shows_table_or_empty_state(page: Page):
    page.goto(BASE_URL)
    has_table = page.locator("table").count() > 0
    has_empty = page.locator("text=Няма намерени медии").count() > 0
    assert has_table or has_empty, "Expected table or empty-state message"


def test_list_has_tree_navigation_link(page: Page):
    page.goto(BASE_URL)
    expect(page.locator("a[href='/tree']")).to_be_visible()


def test_list_has_scan_button_in_navbar(page: Page):
    page.goto(BASE_URL)
    expect(page.locator("button[type=submit]", has_text="Сканирай")).to_be_visible()


def test_actions_has_five_status_buttons(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    row = page.locator("tbody tr").first
    assert row.locator("button.icon-btn").count() == 5


def test_filename_cell_is_button_styled_as_link(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    btn = page.locator("td.filename button.filename-link").first
    expect(btn).to_be_visible()
    assert btn.text_content().strip() != "", "Filename button should have text"


def test_folder_column_is_visible(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    expect(page.locator("td.folder").first).to_be_visible()


def test_detail_link_exists_in_actions(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    expect(page.locator("a.icon-btn[title='Детайли']").first).to_be_visible()


# ---------------------------------------------------------------------------
# Стартиране на медия
# ---------------------------------------------------------------------------

def test_clicking_filename_redirects_back_to_list(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("td.filename button.filename-link").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    assert "/media/" not in page.url


def test_start_media_does_not_show_error(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("td.filename button.filename-link").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


# ---------------------------------------------------------------------------
# Смяна на статус (icon buttons)
# ---------------------------------------------------------------------------

def test_status_change_started_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("button.icon-btn[title='Стартирана']").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


def test_status_change_completed_both_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("button.icon-btn[title='Завършена (двамата)']").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


def test_status_change_to_new_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("button.icon-btn[title='Нова']").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


# ---------------------------------------------------------------------------
# Детайли
# ---------------------------------------------------------------------------

def test_detail_link_opens_detail_page(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("a.icon-btn[title='Детайли']").first.click()
    expect(page.locator("h1")).to_be_visible()
    assert "/media/" in page.url


# ---------------------------------------------------------------------------
# Сканиране
# ---------------------------------------------------------------------------

def test_scan_button_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    page.locator("button[type=submit]", has_text="Сканирай").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    assert page.url.rstrip("/").endswith("8080") or page.url.endswith("/")
