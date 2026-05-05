from playwright.sync_api import Page, expect

BASE_URL = "http://localhost:8080"


def test_page_loads(page: Page):
    page.goto(BASE_URL)
    expect(page.locator("h1")).to_contain_text("Movie Tracker")


def test_scan_button_visible(page: Page):
    page.goto(BASE_URL)
    btn = page.locator("button[type=submit]", has_text="Сканирай")
    expect(btn).to_be_visible()
