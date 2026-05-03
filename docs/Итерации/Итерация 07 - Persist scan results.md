# Итерация 07: Persist scan results

## Цел

Синхронизирай резултатите от файловия сканер със SQLite базата. Scanner находки се запазват в `media` таблицата чрез sqlc-генериран upsert, а файловете които липсват от скана се маркират като `off_disk`.

## Архитектура

### Pattern: Mark-All-Off → Upsert-Found

1. **MarkAllOffDisk** — UPDATE SET `on_disk = 0` за всички редове (преди sync)
2. **UpsertMedia** — INSERT ... ON CONFLICT → UPDATE за всеки намерен файл (с `on_disk = 1`)
3. Файловете които не са в скана остават с `on_disk = 0` (изчезнали със диска)

**Защо този pattern:**
- Просто и разбираемо
- Безопасен при crash (следващ стартиране пренаправя пълния sync)
- Без транзакция обвиване (не е нужна за този use case)

### Идентификация на медия

Уникалност по `(filename, file_size_bytes)`:
- `filename` — базово име на файл (e.g. "movie.mkv")
- `file_size_bytes` — размер в байтове (стабилен при преместване)
- `folder_relative_path` — папка спрямо disk root (се обновява при преместване)

## Реализирани файлове

### 1. SQL заявки — `internal/db/queries.sql`

Три sqlc-анотирани заявки:

```sql
-- name: UpsertMedia :one
INSERT INTO media (filename, folder_relative_path, file_size_bytes, on_disk)
VALUES (?, ?, ?, 1)
ON CONFLICT(filename, file_size_bytes) DO UPDATE SET
    folder_relative_path = excluded.folder_relative_path,
    on_disk = 1
RETURNING id;

-- name: MarkAllOffDisk :exec
UPDATE media SET on_disk = 0;

-- name: CountOnDisk :one
SELECT COUNT(*) FROM media WHERE on_disk = 1;
```

**Генериран код:**
- `func (q *Queries) UpsertMedia(ctx context.Context, arg UpsertMediaParams) (int64, error)`
- `func (q *Queries) MarkAllOffDisk(ctx context.Context) error`
- `func (q *Queries) CountOnDisk(ctx context.Context) (int64, error)` (test helper)
- `type UpsertMediaParams { Filename, FolderRelativePath string; FileSizeBytes int64 }`

### 2. Sync функция — `internal/db/sync.go`

```go
package db

func SyncScanResults(database *sql.DB, files []scanner.VideoFile) error
```

**Логика:**
1. Създай sqlc Queries instance чрез `New(database)`
2. Извикай `MarkAllOffDisk()` за да маркираш всички като off-disk
3. Loop чрез `files`, за всеки извикай `UpsertMedia()` с mapping:
   - `f.Filename` → `Filename`
   - `f.FolderRelativePath` → `FolderRelativePath`
   - `f.SizeBytes` → `FileSizeBytes`
4. На error, оберни със `fmt.Errorf` и върни

**Без транзакция:** Частичен crash (e.g. крах след MarkAllOffDisk но преди всички upserts) оставя някои файлове с `on_disk = 0` и някои с `on_disk = 1`. При следващ стартиране, пълния sync пренаправя всичко правилно. Това е безопасно и прости.

### 3. Тестове — `internal/db/sync_test.go`

Пет TDD тестове (test-first, после имплементация):

| Тест | Проверява |
|------|-----------|
| **TestSyncScanResults_InsertsNewFiles** | 2 новых файла → COUNT(on_disk=1) = 2 |
| **TestSyncScanResults_UpsertUpdatesFolder** | Re-scan със различен folder path → row обновен |
| **TestSyncScanResults_MarksRemovedFilesOffDisk** | Файл B изчезва от scan → on_disk=0; A остава on_disk=1 |
| **TestSyncScanResults_EmptyScanMarksAllOffDisk** | Празен scan (диск изключен) → всички on_disk=0 |
| **TestSyncScanResults_Idempotent** | Двоен scan → без дублирания, COUNT(*)=2, всички on_disk=1 |

Всички использвают helper `openTestDB(t)` който:
- Отваря DB в t.TempDir()
- Прилага миграции
- Връща готова за тест инстанция

### 4. Wire-up — `cmd/movietracker/main.go`

След `scanner.Scan` и лог логика, преди gin router:

```go
if err := db.SyncScanResults(d, files); err != nil {
    log.Fatalf("sync scan results failed: %v", err)
}
log.Printf("Synced %d video files to database", len(files))
```

### 5. Build система — `Taskfile.yml`

Обновиране на `generate` task:

```yaml
generate:
  desc: Generate templ and sqlc code
  cmds:
    - sqlc generate
```

(Премахни placeholder, остави само sqlc generate)

## Верификация стъпки

1. ✏️ Запиши queries в `queries.sql` (три заявки)
2. 🔧 Генерирай sqlc: `sqlc generate` → файл `internal/db/query.sql.go`
3. ✅ Напиши тестове в `sync_test.go` (TDD red phase)
4. ✅ Напиши sync.go (TDD green phase)
5. 🔗 Wire-up в main.go
6. 🧪 `go test ./internal/db/...` → 7 тестове (2 стари + 5 нови)
7. 🧪 `go test ./...` → всички пакети
8. 🏗️ `go build -o movietracker.exe ./cmd/movietracker`
9. 🚀 Ръчен тест: стартирай binary → log "Synced N video files"
10. 📝 Commit: `Итерация 07: Persist scan results — upsert + on_disk sync`

## Critical files

- `internal/db/queries.sql` (обновен)
- `internal/db/sync.go` (нов)
- `internal/db/sync_test.go` (нов)
- `cmd/movietracker/main.go` (обновен)
- `Taskfile.yml` (обновен)

## Решения и обоснования

1. **Mark-all-off pattern вместо per-file tracking:**
   - Причина: Просто, не трябва да съхраняваме предишния scan за сравнение
   - Alternative: Сравни с предишния scan (по-сложно)

2. **Без транзакция:**
   - Причина: Частичен crash е безопасен, следващ стартиране поправя
   - Trade-off: Може да има временно несъответствие в DB при crash, но никога данни загуба

3. **CountOnDisk като sqlc query:**
   - Причина: Type-safe, test-friendly, уникално COUNT значение
   - Alternative: Raw SQL в тест (по-слаб, по-лесен за грешка)

4. **UpsertMediaParams със PascalCase полета:**
   - sqlc автоматично конвертира snake_case → PascalCase
   - `file_size_bytes` → `FileSizeBytes`, `folder_relative_path` → `FolderRelativePath`

## Следваща итерация

Итерация 08: Web list display. Ще напишем handler който query-ва media от DB и показва списък в браузъра със statuses.
