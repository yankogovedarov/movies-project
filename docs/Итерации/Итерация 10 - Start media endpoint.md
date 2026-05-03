# Итерация 10: Start media endpoint

## Цел

`POST /media/:id/start` записва start event безусловно, обновява статус `new → started` и стартира VLC. Добавя бутон "Пусни" към всеки ред в списъка.

## Архитектура

### Рефакторинг: Handlers struct

```go
type Handlers struct {
    DB       *sql.DB
    DiskRoot string
    VLCPath  string
}
func (h *Handlers) Index(c *gin.Context)
func (h *Handlers) StartMedia(c *gin.Context)
```

Handlers вече се нуждаят от DiskRoot (за пълен path) и VLCPath — struct е по-чист от closure параметри.

### StartMedia логика

1. Parse `:id` → 404 при невалиден или несъществуващ
2. `InsertStartEvent(media_id)` — **безусловно** (дори при VLC грешка)
3. Ако `current_status == "new"` → `InsertStatusChange` + `UpdateMediaStatus("started")`
4. Ако `VLCPath != ""` → `exec.Command(vlcPath, fullPath).Start()` (non-blocking)
5. `303 SeeOther → /`

**При липсващ VLC:** събитието пак се записва, VLC не се стартира. Тестовете ползват `VLCPath: ""`.

### Нови SQL заявки

```sql
-- name: GetMediaByID :one
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at
FROM media WHERE id = ?;

-- name: InsertStartEvent :exec
INSERT INTO start_events (media_id) VALUES (?);

-- name: UpdateMediaStatus :exec
UPDATE media SET current_status = ? WHERE id = ?;

-- name: InsertStatusChange :exec
INSERT INTO status_changes (media_id, from_status, to_status) VALUES (?, ?, ?);
```

### `templates/list.templ` — бутон Пусни

```html
<form method="POST" action={ "/media/" + strconv.FormatInt(m.ID, 10) + "/start" }>
  <button type="submit">Пусни</button>
</form>
```

## TDD тестове

| Тест | Проверява |
|------|-----------|
| `TestStartMedia_RecordsEventAndSetsStarted` | `new` медия → event в DB, статус = `started` |
| `TestStartMedia_RecordsEvent_WhenAlreadyStarted` | `started` медия → event записан, статус непроменен |
| `TestStartMedia_Returns404_ForUnknownID` | id=999999 → 404 |
| `TestIndex_ShowsStartButton` | списъкът съдържа `/start` форма |

## Реализирани файлове

- `internal/db/queries.sql` + `queries.sql.go` (регенериран)
- `internal/web/handlers.go` (Handlers struct + StartMedia)
- `internal/web/handlers_test.go` (обновени + нови тестове)
- `templates/list.templ` + `list_templ.go` (регенериран)
- `cmd/movietracker/main.go`
