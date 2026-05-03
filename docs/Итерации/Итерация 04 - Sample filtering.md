# Итерация 04 — Sample filtering

**Дата:** 2026-05-03
**Статус:** ✅ завършена
**Приблизителен обхват:** 1-2 часа

## Цел

Филтриране на "Sample" видео файлове — файл под 50 MB в папка, в която има поне един по-голям видео файл, се изключва от резултатите.

## TDD подход

Стриктен test-first: failing test → минимална имплементация → green → refactor.

## Файлове за създаване / промяна

### Code
- `internal/scanner/sample.go` — `FilterSamples(files []VideoFile) []VideoFile`
  - Групира файлове по `FolderRelativePath`
  - Изключва файлове < 50 MB ако в папката има файл ≥ 50 MB
- `internal/scanner/sample_test.go` — unit тестове (TDD)
- `internal/scanner/scanner.go` — добавяне на `FilterSamples` извикване преди return

## TDD стъпки

### Стъпка 4.1 — Failing test

В `internal/scanner/sample_test.go`:

```go
const MB = 1024 * 1024

func TestFilterSamples_RemovesSample(t *testing.T) {
    files := []VideoFile{
        {Filename: "Movie.mkv",  SizeBytes: 800 * MB, FolderRelativePath: "Films/Movie"},
        {Filename: "Sample.mkv", SizeBytes: 30 * MB,  FolderRelativePath: "Films/Movie"},
    }
    result := FilterSamples(files)
    assert.Len(t, result, 1)
    assert.Equal(t, "Movie.mkv", result[0].Filename)
}

func TestFilterSamples_KeepsSmallOnlyFolder(t *testing.T) {
    files := []VideoFile{
        {Filename: "Short.mkv", SizeBytes: 20 * MB, FolderRelativePath: "Shorts"},
    }
    result := FilterSamples(files)
    assert.Len(t, result, 1)
}
```

### Стъпка 4.2 — Имплементация

`FilterSamples` в `sample.go`:
1. Групира файлове по `FolderRelativePath`
2. За всяка група находит макс размер
3. Ако макс ≥ 50 MB → изключва < 50 MB от групата
4. Иначе → запазва всички

### Стъпка 4.3 — Test passes (green)

- `go test ./internal/scanner/...` минава

### Стъпка 4.4 — Build & verify

- `go build` → без грешки
- Commit

## Reference файлове

| Файл | Цел |
|------|-----|
| [Архитектура.md](../Архитектура.md) | Секция 6.2 — сканиране на диска (sample filtering) |
| [Проект филми.md](../Проект%20филми.md) | Структура на папките, Sample.mkv |

## Out of scope за тази итерация

- Exclude list — Итерация 05
- Конфигурируем праг (50 MB) — hardcoded за MVP-1
- DB интеграция — Итерация 07

## Verification (Definition of Done)

1. **Unit тестове минават:** `go test ./internal/scanner/...` → PASS
2. **Билдът работи:** `go build -o movietracker.exe ./cmd/movietracker` → без грешки
3. **Commit:** `Итерация 04: Sample filtering — exclude small files with larger siblings`
