# Итерация 08: Web list (templ + Pico.css)

## Цел

Покажи списък на медиите от базата данни при `GET /`, рендиран на сървъра чрез `templ` шаблони и стилизиран с Pico.css. Всяка медия показва: файлово име, папка, размер и статус.

## Архитектура

### Нов SQL query

```sql
-- name: ListOnDiskMedia :many
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at
FROM media
WHERE on_disk = 1
ORDER BY folder_relative_path, filename;
```

Генерира `q.ListOnDiskMedia(ctx) ([]db.Medium, error)`.

### Handler промяна: конструктор с DI

```go
func IndexHandler(database *sql.DB) gin.HandlerFunc {
    return func(c *gin.Context) { ... }
}
```

**Защо:** тестовете изискват inject на test DB без глобален state.

### Пакетна структура

```
templates/
├── list.templ        # templ template (source)
├── list_templ.go     # генериран от templ generate
└── helpers.go        # formatSize, statusClass helper функции
```

- `templates` пакет → импортира `internal/db` за `db.Medium`
- `internal/web` → импортира `templates` + `internal/db`
- Няма циклични зависимости

### Pico.css

Зареждан от CDN (`cdn.jsdelivr.net`) за MVP-1. Embed ще бъде добавен по-късно (Итерация 13 polish или при offline нужда).

## TDD тестове

| Тест | Проверява |
|------|-----------|
| `TestIndexHandler_ReturnsHTML` | Content-Type: text/html, HTTP 200 |
| `TestIndexHandler_ShowsMediaFilenames` | Файловите имена на медиите присъстват в HTML |
| `TestIndexHandler_EmptyDB_ShowsNoMediaMessage` | Празна DB → показва "Няма намерени медии" |

## Реализирани файлове

### `internal/db/queries.sql` — добавена заявка

```sql
-- name: ListOnDiskMedia :many
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at
FROM media
WHERE on_disk = 1
ORDER BY folder_relative_path, filename;
```

### `templates/list.templ`

Пълна HTML страница с:
- `<html lang="bg">`, Pico.css CDN link
- `<h1>Movie Tracker</h1>`
- Таблица с колони: Файл, Папка, Размер, Статус
- Ако няма медии → параграф "Няма намерени медии на диска."

### `templates/helpers.go`

- `func formatSize(bytes int64) string` — форматира байтове в GB/MB с 1 знак след запетая
- `func statusClass(status string) string` — CSS клас по статус

### `internal/web/handlers.go`

- `IndexHandler(database *sql.DB) gin.HandlerFunc`
- Вика `q.ListOnDiskMedia(ctx)`, рендира `templates.ListPage(media)`

### `cmd/movietracker/main.go`

- Обновяване: `r.GET("/", web.IndexHandler(d))`

### `Taskfile.yml`

- `generate` task добавя `templ generate` преди `sqlc generate`

## Верификация

1. `go test ./internal/web/...` → 3 нови теста + 1 стар минават
2. `go test ./...` → всички пакети минават
3. `go build -o movietracker.exe ./cmd/movietracker` → без грешки
4. Ръчен тест: `./movietracker.exe` → `http://localhost:8080/` показва HTML списък с медии
