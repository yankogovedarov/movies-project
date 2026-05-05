# Итерация 15: E2E Setup (Python Playwright)

## Цел

Минимален E2E test infrastructure — Python Playwright стартира реален Chromium браузър, стартира Go сървъра автоматично и верифицира golden path.

## Архитектура

```
tests/e2e/
├── pyproject.toml     # pytest-playwright зависимост
├── conftest.py        # session fixture: build + start + kill сървъра
└── test_list_page.py  # 2 E2E теста
```

### conftest.py — `live_server` fixture (scope=session)

1. `go build -o movietracker_e2e.exe ./cmd/movietracker` от repo root
2. Kill евентуален процес на порт 8080
3. Стартира `movietracker_e2e.exe` като subprocess
4. Polling `http://localhost:8080` до 60 секунди (сканирането на C:\ отнема време)
5. Yield — тестовете вървят
6. Kill процеса + изтрива временния binary

## TDD тестове

| Тест | Проверява |
|------|-----------|
| `test_page_loads` | `<h1>Movie Tracker</h1>` е видим в браузъра |
| `test_scan_button_visible` | Бутонът "Сканирай" е видим |

## Реализирани файлове

- `tests/e2e/pyproject.toml`
- `tests/e2e/conftest.py`
- `tests/e2e/test_list_page.py`
- `Taskfile.yml` — нов task `test:e2e`
- `.gitignore` — `movietracker_e2e.exe`, `__pycache__/`, `.pytest_cache/`

## Верификация

```
cd tests/e2e
pip install .
playwright install chromium
pytest --browser chromium -v
# → 2 passed
```

## Бележки

- Първоначалното стартиране отнема ~15 сек (build + scan на C:\)
- `playwright install chromium` трябва да е изпълнено веднъж преди тестовете
- `movietracker_e2e.exe` е в `.gitignore`
