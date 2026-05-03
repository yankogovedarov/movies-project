# Итерация 05 — Excluded folders

**Дата:** 2026-05-03
**Статус:** ✅ завършена
**Приблизителен обхват:** 1-2 часа

## Цел

Прескачане на определени папки при сканиране (Games, Install, Temp и др.). Списъкът е hardcoded за MVP-1 — ще се замени с конфигурационен файл в Итерация 21.

## TDD подход

Стриктен test-first: failing test → минимална имплементация → green → refactor.

## Файлове за създаване / промяна

### Code
- `internal/scanner/scanner.go` — обновяне на `Scan` сигнатура на `Scan(root string, excludeDirs []string) ([]VideoFile, error)`
  - При навлизане в директория: проверява дали `filepath.Base(path)` е в `excludeDirs` → `fs.SkipDir`
- `internal/scanner/scanner_test.go` — добавяне на тест за excluded folder
- `cmd/movietracker/main.go` — подава hardcoded exclude list на `Scan`

## TDD стъпки

### Стъпка 5.1 — Failing test

Добавяме в `internal/scanner/scanner_test.go`:

```go
func TestScan_ExcludesFolder(t *testing.T) {
    root := t.TempDir()
    os.MkdirAll(filepath.Join(root, "Games"), 0755)
    os.WriteFile(filepath.Join(root, "Games", "game.mkv"), make([]byte, 1024), 0644)
    os.WriteFile(filepath.Join(root, "movie.mkv"), make([]byte, 1024), 0644)

    files, err := scanner.Scan(root, []string{"Games"})
    assert.NoError(t, err)
    assert.Len(t, files, 1)
    assert.Equal(t, "movie.mkv", files[0].Filename)
}
```

### Стъпка 5.2 — Имплементация

Обновяваме `Scan` функцията да проверява excluded директории и да ги пропускат с `fs.SkipDir`.

### Стъпка 5.3 — Test passes (green)

- `go test ./internal/scanner/...` минава

### Стъпка 5.4 — Build & verify

- Обновяваме `main.go` да подава hardcoded списък
- `go build` → без грешки
- Commit

## Hardcoded списък (за MVP-1)

`$RECYCLE.BIN`, `System Volume Information`, `found.000`, `Books`, `Download`, `Games`, `Install`, `LizaWork`, `Music`, `Sub`, `Tatko`, `Temp`, `zzz`

## Reference файлове

| Файл | Цел |
|------|-----|
| [Проект филми.md](../Проект%20филми.md) | Списък с изключени папки |

## Out of scope за тази итерация

- Конфигурационен файл — Итерация 21
- Dynamic loading на списък — Итерация 21

## Verification (Definition of Done)

1. **Unit тестове минават:** `go test ./internal/scanner/...` → PASS
2. **Всички тестове минават:** `go test ./...` → PASS
3. **Билдът работи:** `go build -o movietracker.exe ./cmd/movietracker` → без грешки
4. **Commit:** `Итерация 05: Excluded folders — skip hardcoded directories during scan`
