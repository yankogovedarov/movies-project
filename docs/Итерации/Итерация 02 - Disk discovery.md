# Итерация 02 — Disk discovery

**Дата:** 2026-05-03
**Статус:** ✅ завършена
**Приблизителен обхват:** 1-2 часа

## Цел

`internal/disk` пакет, който при стартиране открива disk root и app folder чрез `os.Executable()`. `main.go` го използва и логва намерените пътища.

## TDD подход

Стриктен test-first: failing test → минимална имплементация → green → refactor.

## Файлове за създаване / промяна

### Code
- `internal/disk/discovery.go` — `Discover(executableFn func() (string, error)) (AppPaths, error)`
  - `AppPaths.AppFolder` = папката на `.exe` (напр. `D:\MovieTracker\`)
  - `AppPaths.DiskRoot` = volume на `.exe` + `\` (напр. `D:\`)
- `internal/disk/discovery_test.go` — unit тест (TDD): инжектира mock функция вместо `os.Executable`
- `cmd/movietracker/main.go` — извиква `disk.Discover(os.Executable)` и логва пътищата; при грешка → `log.Fatal`

## TDD стъпки

### Стъпка 2.1 — Failing test

В `internal/disk/discovery_test.go`:

```go
func TestDiscover(t *testing.T) {
    mockExe := func() (string, error) {
        return `D:\MovieTracker\movietracker.exe`, nil
    }
    paths, err := Discover(mockExe)
    assert.NoError(t, err)
    assert.Equal(t, `D:\MovieTracker\`, paths.AppFolder)
    assert.Equal(t, `D:\`, paths.DiskRoot)
}
```

Тестът ще fail-не защото `Discover` още не съществува (red).

### Стъпка 2.2 — Минимална имплементация

- `filepath.Dir(exe)` + `string(os.PathSeparator)` → `AppFolder`
- `filepath.VolumeName(exe)` + `\` → `DiskRoot`

### Стъпка 2.3 — Test passes (green)

- `go test ./internal/disk/...` минава

### Стъпка 2.4 — Build & manual verify

- `go build -o movietracker.exe ./cmd/movietracker` → без грешки
- Стартиране → в конзолата се вижда `AppFolder` и `DiskRoot`

## Reference файлове

| Файл | Цел |
|------|-----|
| [Архитектура.md](../Архитектура.md) | Секция 6.1 — стартиране на приложението |
| [CLAUDE.md](../../CLAUDE.md) | Конвенции, build команди |

## Out of scope за тази итерация

- Config loading (`%LOCALAPPDATA%\MovieTracker\config.toml`) — Итерация 09
- DB отваряне / миграции — Итерация 06
- Сканиране на диска — Итерации 03-05
- Middleware за "disk not connected" — след като имаме DB и HTTP layer

## Verification (Definition of Done)

1. **Unit тестове минават:** `go test ./internal/disk/...` → PASS
2. **Билдът работи:** `go build -o movietracker.exe ./cmd/movietracker` → без грешки
3. **Manual test:** стартиране на `.exe` → конзолата показва `AppFolder` и `DiskRoot`
4. **Commit:** `Итерация 02: Disk discovery — os.Executable path detection`
