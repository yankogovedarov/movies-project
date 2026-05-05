# Итерация 16: Tree view

## Цел

Дървовиден изглед, организиращ медиите по папкова йерархия. Toggle между flat list и tree view.

## Архитектура

### `internal/tree/tree.go` — нов пакет

```go
type Node struct {
    Name     string
    Children []*Node
    Files    []db.Medium
}
func Build(media []db.Medium) *Node
```

`Build` разцепва `FolderRelativePath` по `filepath.Separator`, изгражда дървото рекурсивно и сортира децата по азбучен ред.

Пакетът е отделен (не в `web`), за да избегне кръгова зависимост: `templates` → `internal/tree` ← `web`.

### `templates/tree.templ` — рекурсивен компонент

`treeNodeComponent(node, depth int)`:
- depth 0 = root (без `<details>` wrapper)
- depth 1 = top-level папки → `<details open>` (отворени по default)
- depth 2+ = подпапки → `<details>` (затворени)

### `GET /tree` в handlers.go

Взима on-disk медии, строи дърво с `tree.Build`, рендира `TreePage`.

### Навигация

- Flat list (`/`): показва "Дърво" link
- Tree view (`/tree`): показва "Списък" link

## TDD тестове

| Пакет | Тест | Проверява |
|-------|------|-----------|
| `internal/tree` | `TestBuild_GroupsFilesByFolder` | 3 файла в 2 папки → 2 node-а |
| `internal/tree` | `TestBuild_NestedFolders` | `DoNotDelete\SciFi` → вложени node-ове |
| `internal/tree` | `TestBuild_SortsChildrenAlphabetically` | node-овете са сортирани |
| `internal/tree` | `TestBuild_EmptyMedia` | nil → root без деца |
| `internal/tree` | `TestBuild_RootLevelFiles` | файл с empty folder → в root |
| `internal/web` | `TestTreeHandler_ReturnsHTML` | GET /tree → 200 + text/html |
| `internal/web` | `TestTreeHandler_ShowsFolderNames` | папките са видими |
| E2E | `test_tree_page_loads` | h1 "Movie Tracker" е видим |
| E2E | `test_tree_has_details_element` | `<details>` елементи или empty state |
| E2E | `test_tree_has_list_navigation_link` | link към "/" е присъстващ |
| E2E | `test_list_has_tree_navigation_link` | link към "/tree" е присъстващ |

## Реализирани файлове

- `internal/tree/tree.go` (нов пакет)
- `internal/tree/tree_test.go` (5 теста)
- `internal/web/handlers.go` (Tree handler)
- `internal/web/handlers_test.go` (2 нови теста + route)
- `templates/tree.templ` (нов)
- `templates/tree_templ.go` (генериран)
- `templates/list.templ` (навигация)
- `templates/list_templ.go` (регенериран)
- `cmd/movietracker/main.go` (route GET /tree)
- `tests/e2e/test_tree_view.py` (4 E2E теста)
