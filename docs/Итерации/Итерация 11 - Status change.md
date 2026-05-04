# Итерация 11: Status change

## Цел

`POST /media/:id/status` променя статус, записва status_change event. Добавя dropdown + бутон за ръчна смяна на статус в списъка.

## Архитектура

### ChangeStatus handler

```go
func (h *Handlers) ChangeStatus(c *gin.Context)
```

1. Parse `:id` → 404 при невалиден
2. `status` form field → новия статус (без валидация — приема всяка стойност)
3. `GetMediaByID` → 404 при несъществуващ
4. Ако `status` е същия като текущия → redirect без промяна
5. `InsertStatusChange(id, currentStatus, newStatus)`
6. `UpdateMediaStatus(id, newStatus)`
7. `303 SeeOther → /`

### UI: Dropdown + Смени бутон

```html
<form method="POST" action="/media/{ id }/status">
  <select name="status">
    <option value="new">Нова</option>
    <option value="started">Стартирана</option>
    <option value="completed_both">Завършена (двамата)</option>
    <option value="completed_yanko">Завършена (Янко)</option>
    <option value="completed_liza">Завършена (Лиза)</option>
  </select>
  <button type="submit">Смени</button>
</form>
```

## TDD тестове

| Тест | Проверява |
|------|-----------|
| `TestChangeStatus_UpdatesAndRecordsChange` | POST с нов статус → DB обновена, event записан |
| `TestChangeStatus_Returns404_ForUnknownID` | id=999999 → 404 |
| `TestChangeStatus_NoOp_WhenSameStatus` | статус същия → redirect, event не се записва |
| `TestIndex_ShowsStatusDropdown` | списъкът съдържа `<select name="status">` |

## Реализирани файлове

- `internal/web/handlers.go` (добавена ChangeStatus)
- `internal/web/handlers_test.go` (нови тестове)
- `templates/list.templ` (dropdown + Смени бутон)
- `templates/list_templ.go` (регенериран)
- `cmd/movietracker/main.go` (wire up маршрута)
