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


def test_status_dropdown_has_five_options(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    options = page.locator("select[name=status]").first.locator("option")
    assert options.count() == 5
    values = [options.nth(i).get_attribute("value") for i in range(5)]
    assert set(values) == {"new", "started", "completed_both", "completed_yanko", "completed_liza"}


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


# ---------------------------------------------------------------------------
# Стартиране на медия
# ---------------------------------------------------------------------------

def test_clicking_filename_redirects_back_to_list(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("td.filename button.filename-link").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    # Трябва да сме на "/" (list), не на грешна страница
    assert "/media/" not in page.url


def test_start_media_does_not_show_error(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("td.filename button.filename-link").first.click()
    # Ако имаше грешка, страницата щеше да покаже статус код или error текст
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


# ---------------------------------------------------------------------------
# Смяна на статус
# ---------------------------------------------------------------------------

def test_status_change_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("td.actions select[name=status]").first.select_option("started")
    page.locator("td.actions button.submit-compact").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


def test_status_change_to_completed_both(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("td.actions select[name=status]").first.select_option("completed_both")
    page.locator("td.actions button.submit-compact").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


def test_status_change_to_new(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("td.actions select[name=status]").first.select_option("new")
    page.locator("td.actions button.submit-compact").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


# ---------------------------------------------------------------------------
# Сканиране
# ---------------------------------------------------------------------------

def test_scan_button_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    page.locator("button[type=submit]", has_text="Сканирай").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    assert page.url.rstrip("/").endswith("8080") or page.url.endswith("/")
