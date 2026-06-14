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


# NB: the list view intentionally has NO view-switch nav link to /tree — those
# links were removed in Bug #16 (Итерация 26); the app runs in list mode while the
# /tree route is kept. Absence on the list page is asserted in
# test_list_interactions.py::test_list_has_no_view_switch_links.
