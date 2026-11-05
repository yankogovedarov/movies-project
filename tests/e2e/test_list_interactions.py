"""
E2E тестове за взаимодействие с flat list изгледа.
Тестовете, изискващи медии, се пропускат ако данните не са налични.
"""
import re

import pytest
from playwright.sync_api import Page, expect

BASE_URL = "http://localhost:8080"

# Bug 32: the row with the last started media above the list. List checks scope
# their selectors to #media-list, so this row is never counted as a list row.
LAST_STARTED = "table.last-started tr#last-started"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _has_media(page: Page) -> bool:
    return page.locator("#media-list td.filename button.filename-link").count() > 0


# ---------------------------------------------------------------------------
# Layout / структура
# ---------------------------------------------------------------------------

def test_list_shows_table_or_empty_state(page: Page):
    page.goto(BASE_URL)
    has_table = page.locator("#media-list table").count() > 0
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
    # 2 status buttons (○ ▶) + 3 flags 👍 👎 ⚠ (Bug 30).
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    row = page.locator("#media-list tbody tr").first
    assert row.locator("td.actions button.icon-btn").count() == 5
    for title in ("За DnD", "За изтриване", "Грешни субтитри"):
        assert row.locator(f"td.actions button.icon-btn[title='{title}']").count() == 1, (
            f"expected the '{title}' flag button in the row"
        )


def test_filename_cell_is_button_styled_as_link(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    btn = page.locator("#media-list td.filename button.filename-link").first
    expect(btn).to_be_visible()
    assert btn.text_content().strip() != "", "Filename button should have text"


def test_folder_column_is_visible(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    expect(page.locator("#media-list td.folder").first).to_be_visible()


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
    page.locator("#media-list td.filename button.filename-link").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    assert "/media/" not in page.url


def test_start_media_does_not_show_error(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("#media-list td.filename button.filename-link").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


# ---------------------------------------------------------------------------
# Смяна на статус (icon buttons)
# ---------------------------------------------------------------------------

def test_status_change_started_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("#media-list button.icon-btn[title='Стартирана']").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


def test_status_change_to_new_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    page.locator("#media-list button.icon-btn[title='Нова']").first.click()
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
    # The real data may already have 👍 raised (the former „За DnD" media), so the
    # first row with the flag lowered is taken. A raised 👎 rules a row out too: 👍
    # would clear it (Bug 33) and this test puts back only 👍.
    lowered = page.locator("#media-list tbody tr").filter(
        has=page.locator("td.actions button.icon-btn[title='За DnD']:not(.filter-active)")
    ).filter(
        has=page.locator("td.actions button.icon-btn[title='За изтриване']:not(.filter-active)")
    )
    if lowered.count() == 0:
        pytest.skip("No media without the 👍 and 👎 flags")
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


def test_like_dislike_exclude_in_row(page: Page):
    # Bug 33: raising 👎 clears 👍 in the same row, without a reload; the lowered
    # flag turns grey and the raised one yellow (computed style).
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    like_selector = "td.actions button.icon-btn[title='За DnD']"
    dislike_selector = "td.actions button.icon-btn[title='За изтриване']"
    # A row with both lowered: clicking would clear a raised flag of the real data,
    # and the cleanup below lowers both.
    lowered = page.locator("#media-list tbody tr").filter(
        has=page.locator(f"{like_selector}:not(.filter-active)")
    ).filter(
        has=page.locator(f"{dislike_selector}:not(.filter-active)")
    )
    if lowered.count() == 0:
        pytest.skip("No media without the 👍 and 👎 flags")
    row_id = lowered.first.get_attribute("id")
    row = page.locator(f"#media-list tr#{row_id}")
    like = row.locator(like_selector)
    dislike = row.locator(dislike_selector)
    active = re.compile(r"\bfilter-active\b")

    try:
        # A marker on window: a full page reload would wipe it.
        page.evaluate("window.__noReload = 1")

        like.click()
        expect(like).to_have_class(active)
        expect(like.locator(".flag-icon")).to_have_css("filter", "none")
        expect(dislike.locator(".flag-icon")).to_have_css("filter", "grayscale(1)")

        dislike.click()
        expect(dislike).to_have_class(active)
        expect(like).not_to_have_class(active)
        expect(dislike.locator(".flag-icon")).to_have_css("filter", "none")
        expect(like.locator(".flag-icon")).to_have_css("filter", "grayscale(1)")

        assert page.evaluate("window.__noReload") == 1, (
            "clicking 👍 or 👎 reloaded the page instead of swapping the row in place"
        )
    finally:
        # Both are lowered also when the test fails half-way (lesson from Iteration
        # 44): the server and its dev database are shared by the whole e2e session
        # and by later runs. value=0 always lowers, so doing it twice is safe.
        for route in ("for-dnd", "for-deletion"):
            page.request.post(
                f"{BASE_URL}/media/{row_id.removeprefix('media-')}/{route}",
                form={"value": "0"},
            )


# ---------------------------------------------------------------------------
# Случаен филм (🎲)
# ---------------------------------------------------------------------------

def test_random_shows_film_in_last_started_row(page: Page):
    # Bug 32: 🎲 no longer writes „🎲 Стартирах: …" — the started film shows in
    # the large row above the list, without a reload, and its list row is
    # updated in place (both out-of-band swaps). The server runs with
    # MOVIETRACKER_NO_LAUNCH=1, so no VLC window opens.
    # Until Bug #28 is fixed this test must run before any test that triggers
    # /scan: a rescan in flight can empty the random pool or drop the row.
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")

    # A stale message (e.g. „Няма филми за избор") must be cleared by the pick.
    page.evaluate("document.getElementById('flash').textContent = 'stale'")
    # A marker on window: a full page reload would wipe it.
    page.evaluate("window.__noReload = 1")
    with page.expect_response(lambda r: "/media/random-new/start" in r.url) as response:
        page.locator("button[title='Стартирай случаен от филтрираните']").click()
    # The pick is random and a filename is not unique (a media is identified by
    # name + size), so the media is found by the id of the OOB row in the response.
    match = re.search(r'<tr id="media-(\d+)"', response.value.text())
    assert match, "expected the chosen media's row in the 🎲 response"
    media_id = match.group(1)
    start_path = f"/media/{media_id}/start"

    last = page.locator(LAST_STARTED)
    expect(last.locator("td.filename form")).to_have_attribute("hx-post", start_path)
    expect(last).to_be_visible()
    expect(page.locator("#flash")).to_have_text("")
    assert page.evaluate("window.__noReload") == 1, (
        "🎲 reloaded the page instead of swapping the rows in place"
    )

    # htmx swaps both rows in one step. If the OOB swap of the list row happened,
    # it already shows the new start count — the same text as after a reload.
    row = page.locator(f"#media-list tr#media-{media_id}")
    expect(row).to_have_count(1)
    start_info = row.locator(".start-info")
    swapped = start_info.inner_text() if start_info.count() else ""
    expect(last.locator(".start-info")).to_have_text(swapped)

    page.reload()
    expect(row).to_have_count(1)
    expect(
        row.locator(".start-info"),
        f"row media-{media_id} was not updated in place by 🎲 (out-of-band swap)",
    ).to_have_text(swapped)
    expect(
        last.locator("td.filename form"),
        "after a reload the row above the list must still show the picked film",
    ).to_have_attribute("hx-post", start_path)


# ---------------------------------------------------------------------------
# Последно стартираният филм над таблицата (Бъг 32)
# ---------------------------------------------------------------------------

def _last_started_media_id(page: Page) -> str | None:
    """The media id of the row above the list, None while it is empty."""
    last = page.locator(LAST_STARTED)
    if last.locator("td").count() == 0:
        return None
    # The name's form posts to /media/<id>/start.
    return last.locator("td.filename form").get_attribute("hx-post").split("/")[2]


def test_start_from_table_updates_last_started_row(page: Page):
    # Bug 32: clicking a name in the list shows that film in the row above the
    # list, without a reload.
    page.goto(BASE_URL)
    if not _has_media(page):
        pytest.skip("No media available")
    current = _last_started_media_id(page)
    # A list row of another media, so that the change is visible. A started one
    # is preferred: starting a new media also moves it to „Старт" for good in the
    # shared dev database, and undoing that via /status would add a
    # status_changes row. Any other row is only the fallback.
    rows = page.locator("#media-list tbody tr").evaluate_all(
        "rows => rows.map(r => [r.id.replace('media-', ''),"
        " r.querySelector('td.status').textContent.trim()])"
    )
    others = [(row_id, status) for row_id, status in rows if row_id != current]
    if not others:
        pytest.skip("No other media in the list")
    started = [row_id for row_id, status in others if status == "Старт"]
    target_id = started[0] if started else others[0][0]

    page.evaluate("window.__noReload = 1")
    page.locator(f"#media-list tr#media-{target_id} button.filename-link").click()

    last = page.locator(LAST_STARTED)
    expect(last.locator("td.filename form")).to_have_attribute(
        "hx-post", f"/media/{target_id}/start"
    )
    expect(last).to_be_visible()
    assert page.evaluate("window.__noReload") == 1, (
        "clicking a name reloaded the page instead of swapping the rows in place"
    )


def test_last_started_row_flag_updates_both_rows(page: Page):
    # Bug 32: 👍 in the row above the list toggles the flag there and in the
    # media's list row, without a reload.
    page.goto(BASE_URL)
    media_id = _last_started_media_id(page)
    if media_id is None:
        pytest.skip("Nothing was started yet")
    row = page.locator(f"#media-list tr#media-{media_id}")
    if row.count() == 0:
        pytest.skip("The last started media is not in the list")
    selector = "td.actions button.icon-btn[title='За DnD']"
    top_button = page.locator(LAST_STARTED).locator(selector)
    row_button = row.locator(selector)
    active = re.compile(r"\bfilter-active\b")
    was_raised = bool(active.search(top_button.get_attribute("class") or ""))
    dislike = page.locator(LAST_STARTED).locator("td.actions button.icon-btn[title='За изтриване']")
    dislike_was_raised = bool(active.search(dislike.get_attribute("class") or ""))

    try:
        page.evaluate("window.__noReload = 1")
        top_button.click()
        if was_raised:
            expect(top_button).not_to_have_class(active)
            expect(row_button).not_to_have_class(active)
        else:
            expect(top_button).to_have_class(active)
            expect(row_button).to_have_class(active)
        assert page.evaluate("window.__noReload") == 1, (
            "clicking 👍 reloaded the page instead of swapping the rows in place"
        )
    finally:
        # The flag is put back also when the test fails half-way (lesson from
        # Iteration 44): the server and its dev database are shared by the
        # whole e2e session and by later runs.
        page.request.post(
            f"{BASE_URL}/media/{media_id}/for-dnd",
            form={"value": "1" if was_raised else "0"},
        )
        # Bug 33: raising 👍 cleared a raised 👎, so that one is put back as well.
        if dislike_was_raised:
            page.request.post(
                f"{BASE_URL}/media/{media_id}/for-deletion",
                form={"value": "1"},
            )


def test_last_started_row_is_larger(page: Page):
    # Bug 32: the name and the buttons of the row above the list are larger than
    # those of a list row.
    page.goto(BASE_URL)
    if _last_started_media_id(page) is None or not _has_media(page):
        pytest.skip("Nothing was started yet or no media in the list")
    last = page.locator(LAST_STARTED)

    font_size = "el => parseFloat(getComputedStyle(el).fontSize)"
    top_font = last.locator("button.filename-link").evaluate(font_size)
    list_font = page.locator("#media-list button.filename-link").first.evaluate(font_size)
    assert top_font >= 1.2 * list_font, (
        f"name font {top_font}px above the list should be larger than {list_font}px"
    )

    top_box = last.locator("button.icon-btn").first.bounding_box()
    list_box = page.locator("#media-list button.icon-btn").first.bounding_box()
    assert top_box is not None and list_box is not None
    assert top_box["height"] >= 1.2 * list_box["height"], (
        f"icon button height {top_box['height']} above the list should be larger "
        f"than {list_box['height']}"
    )


# ---------------------------------------------------------------------------
# Сканиране
# ---------------------------------------------------------------------------

def test_scan_button_redirects_to_list(page: Page):
    page.goto(BASE_URL)
    page.locator("button.icon-btn[title='Сканирай диска']").first.click()
    expect(page.locator("h1")).to_contain_text("Movie Tracker")
    assert page.url.rstrip("/").endswith("8080") or page.url.endswith("/")
