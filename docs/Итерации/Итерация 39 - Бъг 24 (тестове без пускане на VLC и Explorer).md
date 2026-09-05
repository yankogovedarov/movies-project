# Итерация 39 — Бъг 24: тестовете да не пускат VLC и Explorer

## Проблем

Пускането на тестовата свита отваря външни прозорци, които после трябва да се затварят ръчно:

| Къде | Тест | Какво се отваря |
|------|------|-----------------|
| Go | `TestOpenFolder_Returns303` | `explorer.exe` |
| Go | `TestOpenFolder_HTMX_Returns200` | `explorer.exe` |
| E2E | `test_clicking_filename_redirects_back_to_list` | VLC |
| E2E | `test_start_media_does_not_show_error` | VLC |
| E2E | `test_tree_clicking_filename_redirects_to_list` | VLC |
| E2E | всяка сесия (`conftest.live_server`) | браузър (`openBrowser`) |

`OpenFolder` вика `exec.Command("explorer.exe", …)` **безусловно** — за него няма как да се
избегне пускането отвън. `StartMedia` / `RandomNew` пускат VLC само при `VLCPath != ""`,
затова Go тестовете (с `VLCPath: ""`) са безопасни, но e2e работят срещу истинския
binary с истински открит VLC.

## Решение

Не се трият тестове. Пускането на външна програма става **инжектируемо**, а бинарникът
получава изключвател през среда:

1. **`web.Launcher`** — `func(name string, args ...string) error`, ново поле на `Handlers`.
   - `web.ExecLauncher` — реалното `exec.Command(name, args...).Start()` (по подразбиране,
     когато полето е `nil` — съществуващият код не се променя поведенчески).
   - `web.NoopLauncher` — не прави нищо.
2. Трите места в `handlers.go` (`RandomNew`, `StartMedia`, `OpenFolder`) минават през
   `h.launch(...)`, който избира launcher-а.
3. **`web.NoLaunch()`** — `true` при `MOVIETRACKER_NO_LAUNCH=1`. `main.go` при нея подава
   `NoopLauncher` **и** пропуска `openBrowser`.
4. Go тестовете подават recording launcher в `newRouter` → нищо не се стартира, а
   покритието се засилва (проверява се *какво* щеше да се пусне).
5. `tests/e2e/conftest.py` стартира бинарника с `MOVIETRACKER_NO_LAUNCH=1` →
   нито VLC, нито Explorer, нито браузър.

## TDD стъпки

1. **Red:** `TestOpenFolder_UsesLauncher`, `TestStartMedia_LaunchesVLC`,
   `TestStartMedia_NoVLCPath_DoesNotLaunch`, `TestNoLaunch_*` — не компилират / падат.
2. **Green:** `Launcher`, `ExecLauncher`, `NoopLauncher`, `h.launch`, `NoLaunch`, промени
   в `main.go` и `conftest.py`.
3. Пълна свита: `task test` + `task test:e2e` зелени, без отворен прозорец.

## Обхват

- `internal/web/handlers.go`, `internal/web/handlers_test.go`
- `cmd/movietracker/main.go`
- `tests/e2e/conftest.py`
- Версия → `v1.8.1` (Taskfile)
- Документация: `Открити бъгове.md`, `Архитектура.md`, `README.md`
