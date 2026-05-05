# Итерация 13: MVP-1 Polish

## Цел

Финална итерация за MVP-1. Три подобрения:

1. **Structured logging** — замяна на `log.Printf` с `slog` + `lumberjack` (ротиран лог файл в AppFolder)
2. **Logger в handlers** — handlers получават `*slog.Logger` и логват ключови операции
3. **Автоматично отваряне на браузър** — при стартиране приложението отваря `http://localhost:8080` (Bug #6)

## Архитектура

### 1. slog + lumberjack setup в main.go

```go
logFile := &lumberjack.Logger{
    Filename:   filepath.Join(paths.AppFolder, "logs", "app.log"),
    MaxSize:    10,  // MB
    MaxBackups: 3,
    MaxAge:     90,  // дни
    Compress:   true,
}
w := io.MultiWriter(os.Stdout, logFile)
slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
    Level: slog.LevelInfo,
})))
```

Логовете се пишат едновременно на `stdout` и в `logs/app.log`.

### 2. Logger в Handlers struct

```go
type Handlers struct {
    DB       *sql.DB
    DiskRoot string
    VLCPath  string
    Log      *slog.Logger
}
```

Ако `Log == nil` → handlers използват `slog.Default()`. Това позволява тестовете да не се занимават с logger.

### 3. Logging в handlers

- `StartMedia` → `Log.Info("start media", "id", id, "file", media.Filename)`
- `ChangeStatus` → `Log.Info("status change", "id", id, "from", old, "to", new)` (само при реална промяна)
- `Scan` → логва се в горутината: `Log.Info("scan complete", "files", len(files))`

### 4. Автоматично отваряне на браузъра

```go
func openBrowser(url string) {
    _ = exec.Command("cmd", "/c", "start", url).Start()
}
```

Извиква се в main.go след `r.Run()` като горутина:
```go
go openBrowser("http://localhost:8080")
r.Run(":8080")
```

## TDD тестове

| Тест | Проверява |
|------|-----------|
| `TestHandlers_WorkWithNilLogger` | Handlers с `Log: nil` не crash-ват при Index, StartMedia, ChangeStatus, Scan |

Logging е страничен ефект — не се тества директно. Тестът проверява само че nil logger не чупи handlers.

## Реализирани файлове

- `cmd/movietracker/main.go` (slog setup, openBrowser)
- `internal/web/handlers.go` (Log field, logging в handlers)
- `internal/web/handlers_test.go` (TestHandlers_WorkWithNilLogger)
- `go.mod` / `go.sum` (добавяне на `gopkg.in/natefinish/lumberjack.v2`)

## Верификация

1. `go mod tidy` → без грешки
2. `go test ./...` → всички тестове pass
3. `go build` → без грешки
4. Ръчен тест:
   - Стартирай `.exe` → браузърът се отваря автоматично
   - В AppFolder/logs/app.log → лог файл се създава
   - Натисни "Сканирай" → в логовете се вижда "scan complete: N files"
