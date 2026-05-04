# Итерация 12: Background scan

## Цел

`POST /scan` стартира фоново сканиране на диска без блокиране на UI. Добавя бутон "Преканирай" в списъка, който инициира асинхронно сканиране и потребителят остава на главната страница.

## Архитектура

### Scan handler

```go
func (h *Handlers) Scan(c *gin.Context) {
    go func() {
        files, _ := scanner.Scan(h.DiskRoot, excludedDirs)
        _ = db.SyncScanResults(h.DB, files)
        log.Printf("Background scan complete: %d files", len(files))
    }()
    c.Redirect(http.StatusSeeOther, "/")
}
```

**Ключови моменти:**
1. Горутина стартува без чакане — веднага редирект на `/`
2. Сканирането се извършва асинхронно в фон
3. Грешки при сканиране се логват, не влияят на HTTP отговора
4. Исключённи папки използват същата переменна както в main.go

### UI: Бутон Преканирай

```html
<form method="POST" action="/scan">
  <button type="submit">Преканирай</button>
</form>
```

Размещава се в заголовката (до h1 "Movie Tracker"), за да е видим винаги.

### Маршрут

```go
r.POST("/scan", h.Scan)
```

## TDD тестове

| Тест | Проверява |
|------|-----------|
| `TestScan_ReturnsRedirect` | POST /scan → 303 SeeOther към / |
| `TestScan_DoesNotBlockRequest` | Горутина стартува, endpoint отговаря бързо (<100ms) |
| `TestIndex_ShowsScanButton` | Главната страница съдържа бутон "Преканирай" с action="/scan" |

## Реализирани файлове

- `internal/web/handlers.go` (нов Scan handler)
- `internal/web/handlers_test.go` (нови тестове)
- `templates/list.templ` (бутон Преканирай в заголовката)
- `templates/list_templ.go` (регенериран)
- `cmd/movietracker/main.go` (регистрация на маршрута r.POST("/scan", h.Scan))

## Верификация

1. `go test ./internal/web/...` → всички тестове pass
2. `go build` → без грешки
3. Ръчен тест:
   - Главната страница съдържа бутон "Преканирай"
   - Клик на бутон → редирект на `/`
   - Логове показват "Background scan complete: N files"
