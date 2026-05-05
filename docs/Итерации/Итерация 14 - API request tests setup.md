# Итерация 14: API Request Tests Setup (goquery)

## Цел

Добавяне на `goquery` за DOM-базирано тестване на HTML отговорите. Съществуващите тестове използват `assert.Contains` за стрингове — работещо, но крехко (минава дори ако текстът е в `<script>` или атрибут). Goquery позволява точни CSS-selector заявки към реалния DOM.

## Разлика спрямо предишните тестове

```go
// Преди (крехко):
assert.Contains(t, body, "Godzilla.mkv")

// С goquery (точно):
btn := doc.Find("td.filename form button.filename-link")
assert.Equal(t, "Godzilla.mkv", strings.TrimSpace(btn.Text()))
```

## TDD тестове

| Тест | Проверява |
|------|-----------|
| `TestIndex_TableRowCount` | 3 медии → 3 `<tr>` в `<tbody>` |
| `TestIndex_FilenameCell_IsLinkButton` | `td.filename form button.filename-link` съдържа Filename |
| `TestIndex_FolderCell_ShowsPath` | `td.folder` съдържа FolderRelativePath |
| `TestIndex_StartFormAction` | form action в `td.filename` съдържа `/start` |
| `TestIndex_StatusFormAction` | form action в `td.actions` съдържа `/status` |
| `TestIndex_StatusSelect_HasFiveOptions` | `select[name=status]` има точно 5 `<option>` с правилни values |

## Реализирани файлове

- `internal/web/handlers_goquery_test.go` (нов — 6 теста + `parseBody` helper)
- `go.mod` / `go.sum` (добавяне на `github.com/PuerkitoBio/goquery v1.12.0`)

## Верификация

1. `go test ./internal/web/... -v` → 22 теста минават (16 стари + 6 нови)
2. `go test ./...` → всички пакети минават
