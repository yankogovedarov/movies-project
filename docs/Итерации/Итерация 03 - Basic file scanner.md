# Итерация 03 — Basic file scanner

**Дата:** 2026-05-03
**Статус:** ✅ завършена
**Приблизителен обхват:** 1-2 часа

## Цел

`internal/scanner` пакет, който рекурсивно обхожда дадена папка и връща списък с намерени видео файлове (`.mkv`, `.avi`, `.mp4`). Без DB, без exclude list, без sample filtering — само чистото намиране на файлове.

## TDD подход

Стриктен test-first: failing test → минимална имплементация → green → refactor.

## Файлове за създаване / промяна

### Code
- `internal/scanner/scanner.go` — `Scan(root string) ([]VideoFile, error)`
  - `VideoFile` struct: `Path string` (пълен път), `Filename string`, `SizeBytes int64`, `FolderRelativePath string`
  - Използва `filepath.WalkDir` — поддържа дълги пътища на Windows
  - Разпознава само `.mkv`, `.avi`, `.mp4` (case-insensitive)
- `internal/scanner/scanner_test.go` — unit тест с временна директорна структура (`t.TempDir()`)
- `cmd/movietracker/main.go` — извиква `scanner.Scan(paths.DiskRoot)` и логва броя намерени файлове

## TDD стъпки

### Стъпка 3.1 — Failing test

В `internal/scanner/scanner_test.go`:

```go
func TestScan(t *testing.T) {
    root := t.TempDir()
    // създава: root/Movies/film.mkv, root/Movies/film.avi, root/other.txt
    files, err := Scan(root)
    assert.NoError(t, err)
    assert.Len(t, files, 2)
    assert.Contains(t, files[0].Filename, "mkv")
}
```

### Стъпка 3.2 — Минимална имплементация

- `filepath.WalkDir(root, ...)` → итериране на всички файлове
- Филтърване по разширение: `.mkv`, `.avi`, `.mp4` (case-insensitive)
- Попълване на `VideoFile` структура

### Стъпка 3.3 — Test passes (green)

- `go test ./internal/scanner/...` минава

### Стъпка 3.4 — Build & manual verify

- `go build` → без грешки
- Стартиране → логва `Found N video files`

## Reference файлове

| Файл | Цел |
|------|-----|
| [Архитектура.md](../Архитектура.md) | Секция 6.2 — сканиране на диска |
| [Проект филми.md](../Проект%20филми.md) | Структура на диска, видео формати |

## Out of scope за тази итерация

- Exclude list (hardcoded папки) — Итерация 05
- Sample filtering (< 50 MB) — Итерация 04
- DB upsert — Итерация 07
- Дълги пътища (`\\?\`) — по-късна итерация при нужда
- Background goroutine — Итерация 12

## Verification (Definition of Done)

1. **Unit тестове минават:** `go test ./internal/scanner/...` → PASS
2. **Билдът работи:** `go build -o movietracker.exe ./cmd/movietracker` → без грешки
3. **Manual test:** стартиране → логва `Found N video files`
4. **Commit:** `Итерация 03: Basic file scanner — recursive directory traversal`
