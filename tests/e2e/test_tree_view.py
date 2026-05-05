from playwright.sync_api import Page, expect

BASE_URL = "http://localhost:8080"


def test_tree_page_loads(page: Page):
    page.goto(f"{BASE_URL}/tree")
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


def test_tree_has_details_element(page: Page):
    page.goto(f"{BASE_URL}/tree")
    details_count = page.locator("details").count()
    empty_count = page.locator("text=Няма намерени медии").count()
    assert details_count > 0 or empty_count > 0, "expected details elements or empty message"


def test_tree_has_list_navigation_link(page: Page):
    page.goto(f"{BASE_URL}/tree")
    expect(page.locator("a[href='/']")).to_be_visible()


def test_list_has_tree_navigation_link(page: Page):
    page.goto(BASE_URL)
    expect(page.locator("a[href='/tree']")).to_be_visible()
