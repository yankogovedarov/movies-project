"""
E2E тестове за филтри по статус и наличност (/?status=...&disk=...).
"""
import re

import pytest
from playwright.sync_api import Page, expect

BASE_URL = "http://localhost:8080"


def test_filter_controls_visible(page: Page):
    page.goto(BASE_URL)
    expect(page.locator(".filter-btns")).to_be_visible()
    expect(page.locator("a.filter-btn[title='Всички статуси']")).to_be_visible()


def test_filter_status_buttons_cover_all_statuses(page: Page):
    # Таргетираме конкретните статус бутони по title, за да е сигурно, че
    # status=all идва уникално от „Всички статуси", а не от disk-toggle/превод
    # бутон, който носи текущия статус.
    page.goto(BASE_URL)
    title_to_status = {
        "Всички статуси": "all",
        "Нова": "new",
        "Стартирана": "started",
        "Завършена от двамата": "completed_both",
        "Завършена от Янко": "completed_yanko",
        "Завършена от Лиза": "completed_liza",
    }
    for title, status in title_to_status.items():
        href = page.locator(f"a.filter-btn[title='{title}']").get_attribute("href")
        assert f"status={status}" in href, (
            f"Expected button '{title}' to point to status={status}, got href={href}"
        )


def test_filter_disk_toggle_has_both_directions(page: Page):
    # При disk=on (по подразбиране) toggle ↕ сочи към disk=all.
    page.goto(BASE_URL)
    toggle = page.locator("a.filter-btn[title*='кликни']")
    expect(toggle).to_have_count(1)
    assert "disk=all" in toggle.get_attribute("href")

    # При disk=all toggle ↕ сочи обратно към disk=on.
    page.goto(f"{BASE_URL}/?disk=all")
    toggle = page.locator("a.filter-btn[title*='кликни']")
    expect(toggle).to_have_count(1)
    assert "disk=on" in toggle.get_attribute("href")


def test_filter_by_status_updates_url(page: Page):
    page.goto(BASE_URL)
    page.locator("a.filter-btn[title='Стартирана']").click()
    assert "status=started" in page.url


def test_filter_by_status_new_shows_results(page: Page):
    page.goto(f"{BASE_URL}/?status=new")
    btn = page.locator("a.filter-btn[title='Нова']")
    expect(btn).to_have_class(re.compile(r"\bfilter-active\b"))


def test_filter_disk_toggle_active_when_param_present(page: Page):
    page.goto(f"{BASE_URL}/?disk=all")
    toggle = page.locator("a.filter-btn[title*='кликни']")
    expect(toggle).to_have_class(re.compile(r"\bfilter-active\b"))


def test_search_input_is_live(page: Page):
    # Bug 16: text search works while typing — the input fires on keyup, no Enter.
    page.goto(BASE_URL)
    search = page.locator("input[type=search][name=q]")
    expect(search).to_have_attribute("hx-get", "/")
    expect(search).to_have_attribute("hx-trigger", re.compile(r"keyup"))
    expect(search).to_have_attribute("hx-target", "#media-list")


def test_search_input_is_plain_height(page: Page):
    # Bug 18 part 1: the search field must be plain — about the same height as the
    # other nav-bar controls (filter buttons), not the tall default Pico input.
    page.goto(BASE_URL)
    search_box = page.locator("input[type=search][name=q]").bounding_box()
    filter_box = page.locator("a.filter-btn[title='Всички статуси']").bounding_box()
    assert search_box is not None and filter_box is not None
    # Allow a few px of tolerance (border/rounding), but it must NOT be the ~40px
    # tall default Pico input.
    assert abs(search_box["height"] - filter_box["height"]) <= 6, (
        f"search height {search_box['height']} should match filter button "
        f"height {filter_box['height']}"
    )


def test_search_input_has_no_icon(page: Page):
    # Bug 18: the search field must have no icon — Pico adds a magnifier via
    # background-image on [type=search]; it must be overridden to none.
    page.goto(BASE_URL)
    bg = page.locator("input[type=search][name=q]").evaluate(
        "el => getComputedStyle(el).backgroundImage"
    )
    assert bg == "none", f"search input must have no background icon, got {bg!r}"


def test_for_deletion_filter_button_present(page: Page):
    # Бъг 22: филтърът „за изтриване" живее в съществуващата група .filter-btns.
    page.goto(BASE_URL)
    btn = page.locator(".filter-btns a.filter-btn[title='За изтриване']")
    expect(btn).to_have_count(1)
    assert "del=yes" in btn.get_attribute("href")


def test_for_deletion_filter_active_when_param(page: Page):
    page.goto(f"{BASE_URL}/?del=yes")
    btn = page.locator(".filter-btns a.filter-btn[title='За изтриване']")
    expect(btn).to_have_class(re.compile(r"\bfilter-active\b"))
    # Активният бутон връща обратно към пълния списък.
    assert "del=all" in btn.get_attribute("href")


def test_for_deletion_filter_survives_status_click(page: Page):
    page.goto(f"{BASE_URL}/?del=yes")
    page.locator("a.filter-btn[title='Стартирана']").click()
    assert "del=yes" in page.url
    assert "status=started" in page.url
