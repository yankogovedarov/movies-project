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


def test_list_has_no_view_switch_links(page: Page):
    # Bug 16: view-switch links (Списък/Дърво) are removed from the main menu;
    # the app works in list mode. The /tree route still exists but is not exposed here.
    page.goto(BASE_URL)
    expect(page.locator("nav.top-nav a[href='/tree']")).to_have_count(0)
    expect(page.locator("nav.top-nav a[href='/']")).to_have_count(0)


def test_list_has_scan_button_in_navbar(page: Page):
    page.goto(BASE_URL)
    expect(page.locator("button.icon-btn[title='Сканирай диска']")).to_be_visible()


def test_actions_has_status_and_deletion_buttons(page: Page):
    # 5 статус бутона + 1 „за изтриване" (Бъг 22).
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    row = page.locator("tbody tr").first
    assert row.locator("td.actions button.icon-btn").count() == 6
    assert row.locator("td.actions button.icon-btn[title='За изтриване']").count() == 1


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
    page.locator("button.icon-btn[title='Сканирай диска']").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    assert page.url.rstrip("/").endswith("8080") or page.url.endswith("/")
