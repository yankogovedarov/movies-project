# Итерация 23 — Sync stale E2E tests

## Context

След итерации 18–22 UI-ят се промени (filter/sort бутони вместо `<select>`, icon scan
бутон `↺`, нова колона „превод" с 3 бутона), но E2E свитата (Playwright) остана с
твърдения за стария UI. Текущо състояние:

- **Go свита:** 8 пакета — изцяло зелена.
- **E2E свита:** 11 паднали / 44 минали.

Това е документираният урок в [CLAUDE.md](../../CLAUDE.md): итер. 21/22 промениха UI, но
оставиха stale тестове. Кодът е **правилен** (Go goquery тестовете заключват новия UI като
намерение). Затова **поправяме тестовете да отговарят на реалния DOM**, без промяна в
продукционен код.

## Засегнати тестове (11)

| Файл | Тест(ове) | Счупено сега | Поправка → реален DOM ([list.templ](../../templates/list.templ)) |
|------|-----------|--------------|------------------------------------------------------------------|
| `test_list_page.py` | `test_scan_button_visible` | `button[type=submit]` с текст „Сканирай" | `button.icon-btn[title='Сканирай диска']` (текстът е `↺`) |
| `test_list_interactions.py` | `test_list_has_scan_button_in_navbar`, `test_scan_button_redirects_to_list` | същото | `button.icon-btn[title='Сканирай диска']` |
| `test_list_interactions.py` | `test_actions_has_five_status_buttons` | `tbody tr → button.icon-btn == 5` (сега 8: +3 превод) | `td.actions button.icon-btn == 5` |
| `test_sort.py` | `test_sort_buttons_visible` | count == 4 | count == 6 (name/path/size/last_started/added/marked) |
| `test_filters.py` | 6 теста | `<select name=status>` / `<select name=disk>` + „Филтър" submit | пренаписване към `a.filter-btn[title=…]` + href / `filter-active` проверки |

## Имплементационен план

### 1. `test_list_page.py` — scan бутон

- `test_scan_button_visible`: селектор → `button.icon-btn[title='Сканирай диска']`.

### 2. `test_list_interactions.py` — scan + брой status бутони

- `test_list_has_scan_button_in_navbar`: → `button.icon-btn[title='Сканирай диска']`.
- `test_scan_button_redirects_to_list`: клик на `button.icon-btn[title='Сканирай диска']`,
  после `h1` съдържа „Movie Tracker".
- `test_actions_has_five_status_buttons`: scope до `td.actions` →
  `row.locator("td.actions button.icon-btn").count() == 5` (изключва 3-те превод бутона и
  `a.icon-btn` детайли).

### 3. `test_sort.py` — брой sort бутони

- `test_sort_buttons_visible`: `to_have_count(4)` → `to_have_count(6)`.
  (🎲 е `button.sort-btn`, не `a.sort-btn`, затова не се брои — `a.sort-btn` са точно 6.)

### 4. `test_filters.py` — пренаписване (select → filter бутони)

Текущ UI: статус филтърът е 6 `a.filter-btn` с `title` и `href` съдържащ `status=…`;
наличността е единичен toggle `↕` (`a.filter-btn`, `filter-active` когато `disk=all`).

- `test_filter_controls_visible`: `expect(page.locator(".filter-btns")).to_be_visible()`
  + наличие на `a.filter-btn[title='Всички статуси']`.
- `test_filter_status_select_has_all_options`: за всеки статус
  (`all/new/started/completed_both/completed_yanko/completed_liza`) има `a.filter-btn`
  с `href` съдържащ `status=<стойност>`.
- `test_filter_disk_select_has_on_and_all_options`: toggle `↕` бутонът съществува; при
  `/` (disk=on) `href` сочи `disk=all`, при `/?disk=all` `href` сочи `disk=on`.
- `test_filter_by_status_updates_url`: клик на `a.filter-btn[title='Стартирана']` →
  `status=started` в URL.
- `test_filter_by_status_new_shows_results`: `/?status=new` → бутонът „Нова" има клас
  `filter-active`.
- `test_filter_disk_all_selected_when_param_present`: `/?disk=all` → toggle `↕` има клас
  `filter-active`.

## Ред на имплементация

1. Бързи поправки: `test_sort.py` (count), `test_list_page.py` + scan в `test_list_interactions.py`, status count scope.
2. Пренаписване на `test_filters.py`.
3. Пълна E2E свита зелена; Go свита остава зелена.

## Критични файлове

| Файл | Промяна |
|------|---------|
| `tests/e2e/test_list_page.py` | scan селектор |
| `tests/e2e/test_list_interactions.py` | 2 scan теста + status count scope |
| `tests/e2e/test_sort.py` | count 4 → 6 |
| `tests/e2e/test_filters.py` | пренаписване select → filter бутони |

## Извън обхвата

- Няма E2E покритие за новите sort опции `path` и `size` (active-when-param). Записва се
  за бъдеща итерация — не разширяваме обхвата тук.

## Верификация

1. `python -m pytest tests\e2e --browser chromium` → 55 passed
2. `go test ./...` → зелено
3. `go build -o bin/movietracker.exe ./cmd/movietracker` → компилира

## Резултат и поуки

- **Отклонения от плана:** Три функции в `test_filters.py` бяха преименувани спрямо плана (`test_filter_status_select_has_all_options` → `test_filter_status_buttons_cover_all_statuses`, `test_filter_disk_select_has_on_and_all_options` → `test_filter_disk_toggle_has_both_directions`, `test_filter_disk_all_selected_when_param_present` → `test_filter_disk_toggle_active_when_param_present`), за да отразят новия механизъм (бутони вместо select). Проверката за `status=all` беше затегната — status бутоните се таргетират по `title` атрибут вместо по href съдържание. Иначе планът е изпълнен точно.
- **Ревю находки:** 1 minor: проверката за `status=all` беше хлабава (засягаше и `disk=all`). Адресирано с таргетиране по `title`. 3 nits: имена на функции все още носеха „select" след пренаписването. Адресирано с преименуване на трите функции. Reviewer потвърди, че `tree.templ`/`detail.templ` са недокоснати — tree/detail тестовете остават валидни без странична регресия.
- **Поуки:** UI-зависимите E2E (Playwright) тестове трябва да се обновяват в **същата итерация** като UI промяната, заедно с Go (goquery) тестовете — не в отделна последваща итерация. Имената на тестовите функции са артефакт на механизма: при смяна на UI механизма (напр. `<select>` → бутони) преименувай функциите едновременно с пренаписването, за да не носят остарял речник.
