# Movie Tracker — проект инструкции

Локално уеб приложение за проследяване на гледани филми от външен диск, с автоматично сканиране и стартиране във VLC.

## Основни файлове

| Файл | За какво |
|------|----------|
| [docs/Проект филми.md](docs/Проект%20филми.md) | Пълно задание (контекст, MVP-1 + MVP-2 функционалности, нефункционални изисквания) |
| [docs/Архитектура.md](docs/Архитектура.md) | Стек, DB схема, HTTP endpoints, build workflow, лог на решенията |
| [docs/Гранични случаи.md](docs/Гранични%20случаи.md) | Edge cases (всички решени) |
| [docs/Екип от агенти.md](docs/Екип%20от%20агенти.md) | Multi-agent организация (оркестратор + mt-developer/mt-reviewer/mt-documenter), handoff протокол |

При въпрос или задача, която засяга нещо в спецификацията — провери първо `Проект филми.md`. За имплементационни въпроси — `Архитектура.md`.

## Технологичен стек (избран)

- **Език:** Go 1.22+
- **HTTP:** `gin`
- **DB:** SQLite чрез `modernc.org/sqlite` (pure Go, без CGo)
- **SQL:** `sqlc` (type-safe генериран код от SQL заявки)
- **HTML:** `templ` + HTMX (server-rendered + hypermedia)
- **CSS:** Pico.css
- **Миграции:** `golang-migrate/migrate` (forward-only, embedded в binary)
- **Логване:** `log/slog` + `lumberjack` (ротация)
- **Тестове (Go):** `testing` + `testify` + `goquery` (HTML парсване за API tests)
- **Тестове (E2E):** Python 3.13 + `pytest-playwright` (browser-based в `tests/e2e/`)
- **Build:** Taskfile + `air` (hot reload)
- **Linter:** `golangci-lint` + `lefthook` (pre-commit)

## Ключови архитектурни факти

- **`.exe` живее на самия диск** (в папка `MovieTracker\`). Идентификация на диска чрез `os.Executable()`, без отделен файл-маркер.
- **Идентификация на медия:** `(filename, file_size_bytes)`. Преместване в друга папка не чупи историята.
- **Един `.exe` файл** — статичен binary, без CGo, със `//go:embed` за статични ресурси и миграции.
- **Per-disk данни** (DB, логове) — на самия диск, до `.exe`.
- **Per-computer данни** (VLC път) — в `%LOCALAPPDATA%\MovieTracker\config.toml`.
- **Без AutoPlay** — потребителят винаги стартира `.exe` ръчно.

## Структура на проекта (Go layout)

```
movie-tracker/
├── cmd/movietracker/main.go    # Входна точка
├── internal/
│   ├── config/                 # TOML loading
│   ├── db/                     # sqlc генериран + queries.sql
│   │   └── migrations/         # SQL миграции — embedded оттук чрез //go:embed (000001-000003)
│   ├── scanner/                # Сканиране на диска
│   ├── media/                  # Модел, статуси
│   ├── vlc/                    # VLC интеграция
│   └── web/                    # HTTP handlers + templ templates
├── migrations/                 # ⚠️ DEPRECATED — старо копие на 000001; реалните са в internal/db/migrations/
├── templates/                  # .templ файлове
├── static/                     # Pico.css, HTMX (embedded)
├── tests/                      # Integration tests
├── Taskfile.yml
└── go.mod
```

## Build команди (Taskfile)

| Команда | Описание |
|---------|----------|
| `task install` | Инсталира dev инструменти (templ, sqlc, air, golangci-lint, lefthook) |
| `task generate` | Генерира templ + sqlc код |
| `task lint` | Стартира `gofmt` + `golangci-lint` |
| `task test` | Стартира Go тестове (unit + integration + API) |
| `task test:e2e` | Стартира browser E2E тестове (Python Playwright) |
| `task build` | Билдва статичен `.exe` в `bin/` папката (винаги — никога в корена на проекта) |
| `task run` | Стартира с hot reload (`air`) |
| `task migrate-new -- <name>` | Създава нова миграция |

## Конвенции

- **Език на UI:** български (статусите са "Завършена от двамата" и т.н., в женски род спрямо "медия")
- **Език на код / комити / документация в кода:** английски
- **Комити в git:** на български (стилът на проекта)
- **Терминология:**
  - В техническите файлове (Проект филми.md MVP, Архитектура.md): **"медия"** (включва филм + епизод)
  - В естествен език (Контекст, Дефиниция за готово): **"филм"** (потребителска реч)

## MVP подход

Разделен на **MVP-1** (минимум, end-to-end работещ) и **MVP-2** (доразработка). При имплементация — първо завършваме MVP-1, тестваме с потребителя, после преминаваме към MVP-2.

## Важни поведения

- При промени в [docs/Проект филми.md](docs/Проект%20филми.md), [docs/Архитектура.md](docs/Архитектура.md) или [docs/Гранични случаи.md](docs/Гранични%20случаи.md) — обнови и [README.md](README.md) в същия commit.
- Не дублирай решения между файловете. Ако трябва да се направи промяна, прави я **на едно място** и реферирай.
- При нови решения — добавяй обосновки в "Лог на решенията" в [docs/Архитектура.md](docs/Архитектура.md).

## Environment & Tooling

### Shell & Build
- **Windows platform**: Always use PowerShell for all shell commands (`command` in Bash tool), NEVER Bash. Go commands, git operations, and task runners all run via PowerShell.
- **Go dependency management**: Before any `go` command (test, build, run), check `go.mod` for recent dependency changes and run `go mod tidy` if needed to keep `go.sum` in sync.

### Source Control & Files

**Dangerous files (pre-commit check):**
- Any file matching `*.csv`, `*.parquet`, or larger than 10 MB must be added to `.gitignore` **before staging**. Never stage large data files or binary dumps.
- Verify `.gitignore` is updated before running `git add` or pushing.

**Commit discipline:**
- Always use conventional commit format: `docs:`, `feat:`, `test:`, `chore:`, `fix:` prefixes
- Group logically related changes into a single commit (don't scatter one feature across multiple commits)
- Write clear, present-tense commit messages in Bulgarian for this project
- Example: `feat: добавяне на media list HTML рендиране` or `test: TDD tests за IndexHandler`

### Iteration Workflow
- **Plan first**: Write plan document locally in `docs/Итерации/Итерация NN - <name>.md` **before** starting implementation
- **TDD-strict**: Failing tests first (red), then minimal implementation (green), then refactor if needed
- **One atomic commit per iteration**: Commit plan document + all implementation files together after verification
- **Green before commit**: Run `task test` (Go) **and** `task test:e2e` (browser) before each iteration commit; UI промени трябва да обновяват и съответните Go (goquery) + Playwright тестове. (Урок: итерации 21/22 промениха UI, но оставиха stale тестове — 1 Go + 11 e2e failure-а, защото свитата не е пускана преди комита.)
- Never push intermediate states or partial work — each commit should be a complete, working vertical slice

## Git Janitor Agent

Invoke with `/git-janitor` to automatically:
1. Run `git status` to see all unstaged/untracked changes
2. Stage files by conventional commit type (docs, feat, test, chore, fix)
3. Verify no dangerous files (*.csv, *.parquet, >10MB) are being staged
4. Generate conventional commit messages grouped by type
5. Commit and push to current branch
6. Report push status and confirm success

The janitor runs **without user confirmation** for the staging/commit/push cycle — it only asks for help if merge conflicts arise or dangerous files are detected.
