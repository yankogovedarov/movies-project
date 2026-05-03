# Итерация 06: SQLite + migrations + sqlc

## Цел

Установяване на database слой: SQLite файл в AppFolder, миграции за схемата, type-safe Go код генериран от sqlc. Тази итерация setup-ва инфраструктурата — в Итерация 07 ще се добави реалният upsert на данни.

## Архитектура

- **DB файл:** `movietracker.db` в AppFolder (на същия диск като .exe)
- **Миграции:** forward-only, embedded в binary чрез `//go:embed`, прилагани при стартиране
- **SQL код:** type-safe генериран от `sqlc`, базиран на migrations + queries.sql
- **Водач:** `modernc.org/sqlite` (pure Go, без CGo)
- **Инструменти:** `golang-migrate/migrate` + `sqlc`

## Реализирани файлове

### 1. Database функции — `internal/db/database.go`

**Open(dbPath string) (*sql.DB, error)**
- Отваря SQLite файл (или го създава ако не съществува)
- Ping-ва за да се убеди че е жив
- Wrapped errors с `fmt.Errorf`

**Migrate(db *sql.DB) error**
- Вмъква миграции от embedded `migrations/` папка
- Прилага всички pending миграции (forward-only)
- Безопасно при повторени стартирания (migrate отслежда history)

### 2. Миграции

**migrations/000001_initial.up.sql**
- `media` таблица (id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at)
- `start_events` (id, media_id, user, started_at)
- `status_changes` (id, media_id, user, old_status, new_status, changed_at)
- `tree_state` (id, subtree_path, last_scanned_at)
- Индекси на key columns (current_status, on_disk, folder_relative_path)
- UNIQUE(filename, file_size_bytes) за уникалност на медия

**migrations/000001_initial.down.sql**
- DROP TABLE в обратен ред (tree_state, status_changes, start_events, media)

### 3. sqlc конфигурация — `sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: "sqlite"
    queries: "internal/db/queries.sql"
    schema: "internal/db/migrations/"
    gen:
      go:
        package: "db"
        out: "internal/db"
```

Queries наименование в `internal/db/queries.sql`, schema от `internal/db/migrations/`.

### 4. Заявки (placeholder за сега) — `internal/db/queries.sql`

Съдържа placeholder за бъдещи sqlc заявки.

### 5. Интеграция в main.go

```go
dbPath := filepath.Join(paths.AppFolder, "movietracker.db")
d, err := db.Open(dbPath)  // отваря базата
if err != nil { log.Fatalf(...) }
defer d.Close()

if err := db.Migrate(d); err != nil {  // прилага миграции
    log.Fatalf(...) 
}
```

Базата се инициализира преди сканирането.

## Тестове

**internal/db/database_test.go**

1. **TestOpen** — отваря DB в temp директория, проверява че не е nil и няма error
2. **TestMigrate** — отваря DB, прилага миграции, query-ва че таблица `media` съществува

Всички тестове използват TDD pattern и testify assertions.

## Верификация

✅ `go test ./internal/db/...` — 2/2 тестове минават
✅ `go test ./...` — всички пакети (6 пакета) минават
✅ `go build -o movietracker.exe ./cmd/movietracker` — 0 errors
✅ Binary стартиране → `movietracker.db` се създава в AppFolder

## Решения и обоснования

1. **Embedded миграции чрез //go:embed:**
   - Причина: Single binary deployment без външни файлове
   - Alternative: Четене от диск (по-сложно, повече файлове при deploy)

2. **Forward-only миграции (без down):**
   - Причина: Production databases не трябва да се откатват
   - Решение: down заявките са в .down.sql за dev/testing само

3. **modernc.org/sqlite вместо mattn/go-sqlite3:**
   - Причина: Pure Go, без CGo, по-лесна deploy
   - Trade-off: Малко по-бавно, но достатъчно за този use case

4. **Уникалност по (filename, file_size_bytes):**
   - Причина: Стабилна идентификация при преместване на папки
   - Alternative: Абсолютен path (неустойчив към преместване)

## Следваща итерация

Итерация 07: Scanner → DB sync. Ще добавим sqlc заявки за upsert + синхронизация на `on_disk` флаг.
