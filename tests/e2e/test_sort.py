"""
E2E тестове за сортиране на списъка (/?sort=...) и показване на статистика.
"""
import pytest
from playwright.sync_api import Page, expect

BASE_URL = "http://localhost:8080"


def test_sort_buttons_visible(page: Page):
    page.goto(BASE_URL)
    btns = page.locator("a.sort-btn")
    expect(btns).to_have_count(6)


def test_sort_name_button_active_by_default(page: Page):
    page.goto(BASE_URL)
    active = page.locator("a.sort-btn.filter-active")
    expect(active).to_have_count(1)
    href = active.get_attribute("href")
    assert "sort=name" in href


def test_sort_last_started_button_active_when_param(page: Page):
    page.goto(f"{BASE_URL}/?sort=last_started")
    active = page.locator("a.sort-btn.filter-active")
    expect(active).to_have_count(1)
    href = active.get_attribute("href")
    assert "sort=last_started" in href


def test_sort_added_button_active_when_param(page: Page):
    page.goto(f"{BASE_URL}/?sort=added")
    active = page.locator("a.sort-btn.filter-active")
    expect(active).to_have_count(1)
    href = active.get_attribute("href")
    assert "sort=added" in href


def test_sort_marked_button_active_when_param(page: Page):
    page.goto(f"{BASE_URL}/?sort=marked")
    active = page.locator("a.sort-btn.filter-active")
    expect(active).to_have_count(1)
    href = active.get_attribute("href")
    assert "sort=marked" in href


def test_sort_buttons_preserve_status_filter(page: Page):
    page.goto(f"{BASE_URL}/?status=started&sort=name")
    btns = page.locator("a.sort-btn")
    for i in range(btns.count()):
        href = btns.nth(i).get_attribute("href")
        assert "status=started" in href, f"sort button {i} should preserve status=started in href"


def test_sort_buttons_preserve_disk_filter(page: Page):
    page.goto(f"{BASE_URL}/?disk=all&sort=name")
    btns = page.locator("a.sort-btn")
    for i in range(btns.count()):
        href = btns.nth(i).get_attribute("href")
        assert "disk=all" in href, f"sort button {i} should preserve disk=all in href"


@pytest.mark.parametrize("title,key", [
    ("По дата добавяне", "added"),
    ("По дата завършване", "marked"),
])
def test_date_sort_first_click_is_descending(page: Page, title: str, key: str):
    """Бъг 19: първото натискане на датов сорт бутон сортира низходящо."""
    page.goto(BASE_URL)
    btn = page.locator(f"a.sort-btn[title='{title}']")
    href = btn.get_attribute("href")
    assert f"sort={key}" in href
    assert "dir=desc" in href, f"first click on {key} should sort descending"
    btn.click()
    assert f"sort={key}" in page.url and "dir=desc" in page.url


@pytest.mark.parametrize("title,key", [
    ("По дата добавяне", "added"),
    ("По дата завършване", "marked"),
])
def test_date_sort_second_click_is_ascending(page: Page, title: str, key: str):
    """Бъг 19: второто натискане (активен бутон) обръща посоката на възходяща."""
    page.goto(f"{BASE_URL}/?sort={key}&dir=desc")
    btn = page.locator(f"a.sort-btn[title='{title}']")
    href = btn.get_attribute("href")
    assert f"sort={key}" in href
    assert "dir=asc" in href, f"second click on {key} should sort ascending"


def test_nondate_sort_first_click_is_ascending(page: Page):
    """Регресия: недатовите сорт бутони запазват asc при първо натискане."""
    page.goto(BASE_URL)
    for title in ("По папка/път", "По размер", "По последно стартиране"):
        href = page.locator(f"a.sort-btn[title='{title}']").get_attribute("href")
        assert "dir=asc" in href, f"first click on '{title}' should sort ascending"


def test_sort_by_name_page_loads(page: Page):
    page.goto(f"{BASE_URL}/?sort=name")
    expect(page.locator("table")).to_be_visible()


def test_sort_by_last_started_page_loads(page: Page):
    page.goto(f"{BASE_URL}/?sort=last_started")
    expect(page.locator("table")).to_be_visible()


def test_sort_by_added_page_loads(page: Page):
    page.goto(f"{BASE_URL}/?sort=added")
    expect(page.locator("table")).to_be_visible()


def test_sort_by_marked_page_loads(page: Page):
    page.goto(f"{BASE_URL}/?sort=marked")
    expect(page.locator("table")).to_be_visible()
