# Итерация 18: Filters — по статус и наличност

## Цел

Добавяне на компактни филтри в `GET /` — по статус (`current_status`) и по наличност (`on_disk`). Потребителят избира от два `<select>` елемента и натиска "Филтър" → страницата се презарежда с новите query параметри.

## Мотивация

При много медии (стотици) потребителят иска бързо да стеснява изгледа:  
- "Покажи само стартираните" → `?status=started`  
- "Покажи всички, включително офлайн" → `?disk=all`

## Промени

### SQL (`internal/db/queries.sql`)
- `ListAllMedia :many` — SELECT без WHERE on_disk (за disk=all режим)

### Handler (`internal/web/handlers.go`)
- `Index` чете `?status=` (default `all`) и `?disk=` (default `on`)
- Избира `ListAllMedia` или `ListOnDiskMedia` по disk параметъра
- In-memory статус филтър след зареждане
- Предава `statusFilter, diskFilter` на `ListPage`

### Темплейт (`templates/list.templ`)
- `ListPage` сигнатура: `(media []db.Medium, statusFilter string, diskFilter string)`
- GET форма в `.top-nav` с `select[name=status]` + `select[name=disk]` + submit бутон
- Активната стойност се задава с `selected` атрибут

### Тестове
- 3 нови unit теста в `handlers_test.go`
- 2 нови goquery теста в `handlers_goquery_test.go`
- E2E: `tests/e2e/test_filters.py`

## Дефиниция за готово

- `go test ./...` → всички тестове минават
- `go build` → без грешки
- `/?status=started` показва само стартирани медии
- `/?disk=all` включва офлайн медии
- Смяна на статус с HTMX не чупи страницата при активен филтър
