# Итерация 01 — Walking skeleton

**Дата:** 2026-05-03
**Статус:** в процес
**Приблизителен обхват:** 1-2 часа

## Цел

Минимален работещ Go уеб сървър, който върши Hello World, с пълен dev workflow (build, test, lint, run). Това е foundation върху която ще строим всички следващи итерации.

## TDD подход

Стриктен test-first: failing test → минимална имплементация → green → refactor.

## Файлове за създаване

### Code
- `cmd/movietracker/main.go` — entry point: gin сървър на `:8080` с handler `GET /` → `200 OK` + текст "Movie Tracker"
- `internal/web/handlers.go` — `IndexHandler(c *gin.Context)`
- `internal/web/handlers_test.go` — failing test първо (TDD)

### Конфигурация / билд
- `go.mod` — Go 1.22+, dependency на gin
- `Taskfile.yml` — `task install`, `task generate` (placeholder), `task lint`, `task test`, `task build`, `task run`
- `.gitignore` — допълнения за `*.exe`, `tmp/`, `*.db`
- `.golangci.yml` — minimal config (default linters)
- `.air.toml` — basic hot reload config за `task run`

## TDD стъпки (test-first)

### Стъпка 1.1 — Failing test

В `internal/web/handlers_test.go`:

```go
func TestIndexHandlerReturns200(t *testing.T) {
    router := gin.Default()
    router.GET("/", IndexHandler)
    w := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/", nil)
    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Contains(t, w.Body.String(), "Movie Tracker")
}
```

Този тест ще fail-не защото `IndexHandler` още не съществува (red).

### Стъпка 1.2 — Минимална имплементация

- Създаваме `internal/web/handlers.go` с `IndexHandler` който връща `200 OK` и текст "Movie Tracker"
- `cmd/movietracker/main.go` стартира gin сървъра и регистрира handler-а

### Стъпка 1.3 — Test passes

- `go test ./...` минава (green)
- `task test` минава

### Стъпка 1.4 — Build & manual verify

- `task build` създава `movietracker.exe`
- `task run` стартира приложението
- Браузър на `http://localhost:8080` показва "Movie Tracker"

## Reference файлове

| Файл | Цел |
|------|-----|
| [Архитектура.md](../Архитектура.md) | Структура на проекта (cmd/, internal/), Taskfile дефиниция, тестова стратегия |
| [CLAUDE.md](../CLAUDE.md) | Конвенции, build команди, технологичен стек |
| [Проект филми.md](../Проект%20филми.md) | MVP-1 функционалност (за reference) |

## Out of scope за тази итерация

- DB / SQLite / миграции — следваща итерация
- Sample filter, exclude list, scanner — следващи итерации
- HTML templates / templ / HTMX — по-нататък
- VLC интеграция — по-нататък
- Pre-commit hooks (`lefthook`) — настройваме при първи реален commit
- E2E (Python Playwright) — настройваме когато имаме UI за тест
- `sqlc` config — настройваме при DB итерация

## Verification (Definition of Done)

1. **Unit тестове минават:** `task test` → `PASS` за `internal/web`
2. **Билдът работи:** `task build` създава работещ `movietracker.exe`
3. **Lint минава:** `task lint` без грешки
4. **Manual test:** `task run` → браузър на `http://localhost:8080` показва "Movie Tracker"
5. **Commit:** `Iter 1: Walking skeleton — gin server with hello handler`

## Зависимости

- **Go 1.22+** трябва да е инсталиран на машината (предстои проверка)
- `gin-gonic/gin`, `stretchr/testify` — Go modules dependencies
- `go-task/task` — за изпълнение на Taskfile (one-time install)
- `cosmtrek/air` — за hot reload (one-time install)
- `golangci-lint` — за lint (one-time install)
