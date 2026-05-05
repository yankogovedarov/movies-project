"""
E2E тестове за взаимодействие с дървовидния изглед (/tree).
"""
import pytest
from playwright.sync_api import Page, expect

BASE_URL = "http://localhost:8080"


def _open_first_subfolder(page: Page) -> bool:
    """Отваря първата затворена подпапка за да направи файловете видими."""
    nested_summaries = page.locator("details details summary")
    if nested_summaries.count() == 0:
        return False
    nested_summaries.first.click()
    # Изчакваме файловете да станат видими
    page.wait_for_timeout(200)
    return True


def _has_visible_tree_files(page: Page) -> bool:
    return page.locator(".tree-file button.filename-link").filter(visible=True).count() > 0


# ---------------------------------------------------------------------------
# Layout / структура
# ---------------------------------------------------------------------------

def test_tree_shows_scan_button(page: Page):
    page.goto(f"{BASE_URL}/tree")
    expect(page.locator("button[type=submit]", has_text="Сканирай")).to_be_visible()


def test_tree_has_list_navigation_link(page: Page):
    page.goto(f"{BASE_URL}/tree")
    expect(page.locator("a[href='/']")).to_be_visible()


def test_tree_shows_folders_or_empty_state(page: Page):
    page.goto(f"{BASE_URL}/tree")
    has_folders = page.locator("details").count() > 0
    has_empty = page.locator("text=Няма намерени медии").count() > 0
    assert has_folders or has_empty


def test_tree_top_level_folders_are_open(page: Page):
    page.goto(f"{BASE_URL}/tree")
    if page.locator("details").count() == 0:
        pytest.skip("No folders in tree")
    first_details = page.locator("details").first
    assert first_details.get_attribute("open") is not None, "Top-level folder should be open by default"


def test_tree_has_file_links(page: Page):
    page.goto(f"{BASE_URL}/tree")
    # Файловете може да са в затворени подпапки — отваряме първата
    _open_first_subfolder(page)
    if not _has_visible_tree_files(page):
        pytest.skip("No visible files in tree view")
    btn = page.locator(".tree-file button.filename-link").filter(visible=True).first
    expect(btn).to_be_visible()
    assert btn.text_content().strip() != ""


def test_tree_file_has_status_dropdown(page: Page):
    page.goto(f"{BASE_URL}/tree")
    _open_first_subfolder(page)
    if not _has_visible_tree_files(page):
        pytest.skip("No visible files in tree view")
    options = page.locator(".tree-file select[name=status]").filter(visible=True).first.locator("option")
    assert options.count() == 5


# ---------------------------------------------------------------------------
# Стартиране на медия от дървото
# ---------------------------------------------------------------------------

def test_tree_clicking_filename_redirects_to_list(page: Page):
    page.goto(f"{BASE_URL}/tree")
    _open_first_subfolder(page)
    if not _has_visible_tree_files(page):
        pytest.skip("No visible files in tree view")
    page.locator(".tree-file button.filename-link").filter(visible=True).first.click()
    # StartMedia handler redirect към "/"
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    assert "/tree" not in page.url, "After start from tree, should redirect to list"


# ---------------------------------------------------------------------------
# Смяна на статус от дървото
# ---------------------------------------------------------------------------

def test_tree_status_change_to_started(page: Page):
    page.goto(f"{BASE_URL}/tree")
    _open_first_subfolder(page)
    if not _has_visible_tree_files(page):
        pytest.skip("No visible files in tree view")
    page.locator(".tree-file select[name=status]").filter(visible=True).first.select_option("started")
    page.locator(".tree-file button.submit-compact").filter(visible=True).first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


def test_tree_status_change_to_new(page: Page):
    page.goto(f"{BASE_URL}/tree")
    _open_first_subfolder(page)
    if not _has_visible_tree_files(page):
        pytest.skip("No visible files in tree view")
    page.locator(".tree-file select[name=status]").filter(visible=True).first.select_option("new")
    page.locator(".tree-file button.submit-compact").filter(visible=True).first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


# ---------------------------------------------------------------------------
# Сканиране от дървото
# ---------------------------------------------------------------------------

def test_tree_scan_button_redirects_to_list(page: Page):
    page.goto(f"{BASE_URL}/tree")
    page.locator("button[type=submit]", has_text="Сканирай").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    assert "/tree" not in page.url


# ---------------------------------------------------------------------------
# Expand / collapse на папки
# ---------------------------------------------------------------------------

def test_tree_folder_summary_is_clickable(page: Page):
    page.goto(f"{BASE_URL}/tree")
    if page.locator("details").count() == 0:
        pytest.skip("No folders in tree")
    first_details = page.locator("details").first
    # Отваря се → затваряме с клик (first за да избегнем strict mode при вложени summaries)
    first_details.locator("summary").first.click()
    assert first_details.get_attribute("open") is None, "Folder should be closed after click"


def test_tree_collapsed_folder_can_be_opened(page: Page):
    page.goto(f"{BASE_URL}/tree")
    if page.locator("details:not([open])").count() == 0:
        pytest.skip("No collapsed subfolders in current data")
    # Броим отворените преди и след клика
    open_before = page.locator("details[open]").count()
    page.locator("details:not([open]) summary").first.click()
    open_after = page.locator("details[open]").count()
    assert open_after > open_before, "Clicking collapsed folder should open it"
