"""
E2E тестове за взаимодействие с flat list изгледа.
Тестовете, изискващи медии, се пропускат ако данните не са налични.
"""
import re

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


def test_actions_has_status_and_flag_buttons(page: Page):
    # 2 статус бутона (○ ▶) + 3 независими флага 👍 👎 ⚠ (Бъг 30).
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    row = page.locator("tbody tr").first
    assert row.locator("td.actions button.icon-btn").count() == 5
    for title in ("За DnD", "За изтриване", "Грешни субтитри"):
        assert row.locator(f"td.actions button.icon-btn[title='{title}']").count() == 1, (
            f"expected the '{title}' flag button in the row"
        )


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


def test_list_has_no_details_button(page: Page):
    # Bug 27: the ℹ „Детайли" button and the details view are removed.
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    expect(page.locator("[title='Детайли']")).to_have_count(0)


def test_list_has_no_translation_ui(page: Page):
    # Бъг 30: преводът е махнат — няма група „превод:" в лентата и колона „Пр".
    page.goto(BASE_URL)
    # Лентата не зависи от данните, затова тази част не се пропуска.
    expect(page.locator("nav.top-nav")).not_to_contain_text("превод:")
    expect(page.locator(".trans-btns")).to_have_count(0)
    # Колоната се вижда само в таблицата, т.е. когато има редове.
    if _has_media(page):
        expect(page.locator("th.translation")).to_have_count(0)
        expect(page.locator("td.translation")).to_have_count(0)


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


def test_status_change_to_new_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("button.icon-btn[title='Нова']").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


# ---------------------------------------------------------------------------
# Флагове 👍 👎 ⚠ (Бъг 30)
# ---------------------------------------------------------------------------

def test_dnd_flag_toggles_row_in_place(page: Page):
    # Бъг 30: „За DnD" вече е флаг, а не статус. Кликът на 👍 го вдига на място —
    # HTMX сменя само реда, без презареждане — и колоната „Статус" не се променя;
    # след презареждане бутонът остава подчертан, защото флагът е записан.
    # Вторият клик го сваля: сървърът и базата са общи за цялата e2e сесия.
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    # Реалните данни може вече да имат вдигнат 👍 (бившите „За DnD" медии),
    # затова се взима първият ред със свален флаг.
    lowered = page.locator("tbody tr").filter(
        has=page.locator("td.actions button.icon-btn[title='За DnD']:not(.filter-active)")
    )
    if lowered.count() == 0:
        pytest.skip("No media without the 👍 flag")
    row_id = lowered.first.get_attribute("id")
    row = page.locator(f"tr#{row_id}")
    button = row.locator("td.actions button.icon-btn[title='За DnD']")
    status = row.locator("td.status").inner_text()
    active = re.compile(r"\bfilter-active\b")

    try:
        # Маркер на window: пълно презареждане на страницата би го изтрил.
        page.evaluate("window.__noReload = 1")
        button.click()
        expect(button).to_have_class(active)
        assert page.evaluate("window.__noReload") == 1, (
            "clicking 👍 reloaded the page instead of swapping the row in place"
        )
        expect(row.locator("td.status")).to_have_text(status)

        page.reload()
        expect(button).to_have_class(active)

        button.click()
        expect(button).not_to_have_class(active)
    finally:
        # Флагът се сваля и при провал по средата — иначе 👍 остава вдигнат в
        # общата dev база и следващите пускания тихо взимат друг ред. value=0
        # винаги сваля, затова повторното сваляне е безопасно.
        page.request.post(
            f"{BASE_URL}/media/{row_id.removeprefix('media-')}/for-dnd",
            form={"value": "0"},
        )


# ---------------------------------------------------------------------------
# Случаен филм (🎲)
# ---------------------------------------------------------------------------

def test_random_shows_film_and_updates_row(page: Page):
    # Bug 27: 🎲 names the started film under the nav bar and updates its row in
    # place (out-of-band swap). The server runs with MOVIETRACKER_NO_LAUNCH=1, so
    # no VLC window opens.
    # Until Bug #28 is fixed this test must run before any test that triggers
    # /scan: a rescan in flight can empty the random pool or drop the row.
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")

    with page.expect_response(lambda r: "/media/random-new/start" in r.url) as response:
        page.locator("button[title='Стартирай случаен от филтрираните']").click()
    # The pick is random and a filename is not unique (a media is identified by
    # name + size), so the row is found by the id of the OOB row in the response.
    match = re.search(r'<tr id="(media-\d+)"', response.value.text())
    assert match, "expected the chosen media's row in the 🎲 response"
    row_id = match.group(1)
    row = page.locator(f"tr#{row_id}")
    expect(row).to_have_count(1)
    name = row.locator("button.filename-link").get_attribute("title")

    # Only the film's name — none of the row's content may leak into #flash.
    expect(page.locator("#flash")).to_have_text(f"🎲 Стартирах: {name}")

    # htmx swaps the row and #flash in one step. If the OOB swap happened, the row
    # already shows the new start count — the same text as after a reload.
    start_info = row.locator(".start-info")
    swapped = start_info.inner_text() if start_info.count() else ""
    page.reload()
    expect(row).to_have_count(1)
    expect(
        row.locator(".start-info"),
        f"row {row_id} was not updated in place by 🎲 (out-of-band swap)",
    ).to_have_text(swapped)


# ---------------------------------------------------------------------------
# Сканиране
# ---------------------------------------------------------------------------

def test_scan_button_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    page.locator("button.icon-btn[title='Сканирай диска']").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    assert page.url.rstrip("/").endswith("8080") or page.url.endswith("/")
