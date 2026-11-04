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
    # status=all идва уникално от „Всички статуси", а не от ↕ или флаг бутоните,
    # които носят текущия статус.
    # Бъг 30: статусите са два — ◎ ○ ▶; „За DnD" е флаг (👍), а завършените
    # (✓ Я Л) са махнати още в Бъг 27.
    page.goto(BASE_URL)
    title_to_status = {
        "Всички статуси": "all",
        "Нова": "new",
        "Стартирана": "started",
    }
    for title, status in title_to_status.items():
        href = page.locator(f"a.filter-btn[title='{title}']").get_attribute("href")
        assert f"status={status}" in href, (
            f"Expected button '{title}' to point to status={status}, got href={href}"
        )
    expect(page.locator("a.filter-btn[href*='status=completed']")).to_have_count(0)
    expect(page.locator("a.filter-btn[href*='status=for_dnd']")).to_have_count(0)


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


def test_flag_filter_buttons_present(page: Page):
    # Бъг 30: трите независими флага 👍 👎 ⚠ са в групата .filter-btns и всеки
    # стеснява списъка чрез своя параметър.
    page.goto(BASE_URL)
    for title, icon, param in (
        ("За DnD", "👍", "dnd"),
        ("За изтриване", "👎", "del"),
        ("Грешни субтитри", "⚠️", "subs"),
    ):
        btn = page.locator(f".filter-btns a.filter-btn[title='{title}']")
        expect(btn).to_have_count(1)
        expect(btn).to_have_text(icon)
        href = btn.get_attribute("href")
        assert f"{param}=yes" in href, (
            f"Expected button '{title}' to point to {param}=yes, got href={href}"
        )


def test_filters_persist_after_reopen(page: Page):
    # Бъг 23: избраните филтри и сорт се пазят в базата, така че ново отваряне на
    # голо "/" (както след рестарт на .exe-то) ги възстановява.
    page.goto(f"{BASE_URL}/?status=started&sort=size&dir=desc&del=yes&dnd=yes")

    page.goto(BASE_URL)
    expect(page.locator("a.filter-btn[title='Стартирана']")).to_have_class(
        re.compile(r"\bfilter-active\b")
    )
    expect(page.locator(".filter-btns a.filter-btn[title='За изтриване']")).to_have_class(
        re.compile(r"\bfilter-active\b")
    )
    # Бъг 30: флагът 👍 се помни като 👎.
    expect(page.locator(".filter-btns a.filter-btn[title='За DnD']")).to_have_class(
        re.compile(r"\bfilter-active\b")
    )
    sort_btn = page.locator("a.sort-btn[title='По размер']")
    expect(sort_btn).to_have_class(re.compile(r"\bfilter-active\b"))
    assert sort_btn.inner_text().strip() == "Рз↓"


def test_search_text_persists_after_reopen(page: Page):
    # Решение на потребителя: помни се и текстът от полето „търси…".
    page.goto(f"{BASE_URL}/?q=zzz-няма-такъв-филм")

    page.goto(BASE_URL)
    expect(page.locator("input[type=search][name=q]")).to_have_value("zzz-няма-такъв-филм")


# Бъг 26: падащо меню за основна папка. E2E сканира реалния диск, затова
# тестовете не разчитат на конкретни имена на папки — избират първата папка от
# менюто и се пропускат, ако папки няма.
FOLDER_SELECT = ".filter-btns select[name=folder]"


def _choose_first_folder(page: Page) -> str:
    """Избира първата основна папка от менюто и връща стойността ѝ."""
    # Голо "/" — URL-ът още няма folder=, така че wait_for_url по-долу наистина
    # изчаква презареждането след избора.
    page.goto(BASE_URL)
    select = page.locator(FOLDER_SELECT)
    options = select.locator("option")
    if options.count() < 2:
        pytest.skip("No main folders available")
    chosen = options.nth(1).get_attribute("value")
    select.select_option(index=1)
    # Смяната изпраща формата за търсене — пълно презареждане с folder= в URL-а.
    page.wait_for_url(re.compile(r"[?&]folder="))
    return chosen


def test_folder_select_first_in_filter_group(page: Page):
    # Бъг 26: менюто е първо в групата „филтър:", преди бутона ◎.
    page.goto(BASE_URL)
    select = page.locator(FOLDER_SELECT)
    expect(select).to_be_visible()
    expect(select.locator("option").first).to_have_text("всички папки")
    first = page.locator(".filter-btns").evaluate(
        "el => [el.firstElementChild.tagName, el.firstElementChild.getAttribute('name')]"
    )
    assert first == ["SELECT", "folder"], f"first element in .filter-btns is {first}"


def test_folder_select_is_plain_height(page: Page):
    # Бъг 26: менюто е ниско колкото филтър бутоните, а не високият Pico select.
    page.goto(BASE_URL)
    select_box = page.locator(FOLDER_SELECT).bounding_box()
    filter_box = page.locator("a.filter-btn[title='Всички статуси']").bounding_box()
    assert select_box is not None and filter_box is not None
    assert abs(select_box["height"] - filter_box["height"]) <= 6, (
        f"folder select height {select_box['height']} should match filter button "
        f"height {filter_box['height']}"
    )


def test_folder_select_filters_list(page: Page):
    # Бъг 26: изборът на папка оставя в списъка само медиите под нея.
    chosen = _choose_first_folder(page)

    expect(page.locator(FOLDER_SELECT)).to_have_value(chosen)
    # Менюто предлага само папки с медии, така че списъкът не бива да е празен.
    expect(page.locator("#media-list")).not_to_contain_text("Няма намерени медии.")
    paths = page.locator("#media-list td.folder a.folder-link").evaluate_all(
        "els => els.map(e => e.getAttribute('title'))"
    )
    assert paths, f"expected rows under {chosen!r}"
    for path in paths:
        top = re.split(r"[\\/]", path, maxsplit=1)[0]
        assert top == chosen, f"row in {path!r} is not under the chosen folder {chosen!r}"


def test_live_search_sends_folder(page: Page):
    # Бъг 26: менюто е свързано с формата за търсене (атрибут form), затова
    # живото търсене изпраща и избраната папка.
    _choose_first_folder(page)
    with page.expect_request(lambda r: "q=" in r.url and r.method == "GET") as request:
        page.locator("input[type=search][name=q]").press_sequentially("a")
    assert "folder=" in request.value.url, (
        f"live search request does not carry the folder: {request.value.url}"
    )


def test_folder_filter_persists_after_reopen(page: Page):
    # Бъг 26: избраната папка се помни в базата като останалите филтри.
    chosen = _choose_first_folder(page)

    page.goto(BASE_URL)
    expect(page.locator(FOLDER_SELECT)).to_have_value(chosen)
