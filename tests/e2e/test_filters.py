"""
E2E тестове за филтри по статус и наличност (/?status=...&disk=...).
"""
import pytest
from playwright.sync_api import Page, expect

BASE_URL = "http://localhost:8080"


def test_filter_controls_visible(page: Page):
    page.goto(BASE_URL)
    expect(page.locator("select[name='status']")).to_be_visible()
    expect(page.locator("select[name='disk']")).to_be_visible()


def test_filter_status_select_has_all_options(page: Page):
    page.goto(BASE_URL)
    options = page.locator("select[name='status'] option")
    values = [options.nth(i).get_attribute("value") for i in range(options.count())]
    assert "all" in values
    assert "new" in values
    assert "started" in values
    assert "completed_both" in values
    assert "completed_yanko" in values
    assert "completed_liza" in values


def test_filter_disk_select_has_on_and_all_options(page: Page):
    page.goto(BASE_URL)
    options = page.locator("select[name='disk'] option")
    values = [options.nth(i).get_attribute("value") for i in range(options.count())]
    assert "on" in values
    assert "all" in values


def test_filter_by_status_updates_url(page: Page):
    page.goto(BASE_URL)
    page.select_option("select[name='status']", "started")
    page.locator("button[type='submit']", has_text="Филтър").click()
    assert "status=started" in page.url


def test_filter_by_status_new_shows_results(page: Page):
    page.goto(f"{BASE_URL}/?status=new")
    expect(page.locator("select[name='status']")).to_be_visible()
    selected_val = page.locator("select[name='status']").input_value()
    assert selected_val == "new"


def test_filter_disk_all_selected_when_param_present(page: Page):
    page.goto(f"{BASE_URL}/?disk=all")
    selected_val = page.locator("select[name='disk']").input_value()
    assert selected_val == "all"
