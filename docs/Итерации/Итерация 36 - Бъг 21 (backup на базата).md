# Итерация 36 — Бъг 21: backup на базата при стартиране

## Контекст

[docs/Открити бъгове.md:563](../Открити%20бъгове.md#L563) — **Бъг #21**:

> При пускане на приложението да се прави backup на базата под формата на файл в подпапка
> `backup`, а базата да съдържа името на момента на backup. Например
> `.\backup\2026_08_24_19_07_00_movietracker.db`.

Днес базата (`<AppFolder>\movietracker.db`) няма никаква защита: при повреден файл,
неуспешна миграция или грешна операция цялата история на гледаните филми се губи
безвъзвратно. DB е под 1 MB ([Архитектура.md §Очакван размер](../Архитектура.md#L401)),
така че копие при всяко стартиране е практически безплатно.

**Резултат:** при всеки старт се създава timestamp-нат снапшот на базата **преди** тя да
бъде отворена и мигрирана; пазят се последните 10, по-старите се трият автоматично.

**Решения на потребителя (уточнени):** задържане — **последните 10**; при провал на
backup-а приложението **спира с грешка** (`slog.Error` + `os.Exit(1)`), за да не се
работи никога без пресен снапшот.

## Промени

### 1. Нов `internal/db/backup.go` (пакет `db`)

```go
const BackupKeep = 10          // колко снапшота се пазят
const backupDirName = "backup"
const backupTimeLayout = "2006_01_02_15_04_05"

// Backup копира dbPath в <dir на dbPath>/backup/<timestamp>_<basename>,
// след което изтрива най-старите, докато останат най-много keep файла.
// Ако dbPath не съществува (първи старт) → ("", nil), без да се създава папка.
func Backup(dbPath string, now time.Time, keep int) (string, error)
```

- `os.Stat(dbPath)` → `os.IsNotExist` ⇒ връща `("", nil)` (първи старт, няма какво да се пази).
- `os.MkdirAll(backupDir, 0755)`.
- Копиране с `os.Open` / `os.Create` / `io.Copy` + `Sync()`. Име:
  `now.Format(backupTimeLayout) + "_" + filepath.Base(dbPath)` →
  `2026_08_24_19_07_00_movietracker.db`, точно както е в заданието.
- **Prune:** `os.ReadDir(backupDir)`, филтър само по имена със суфикс `_<basename>` и
  валиден timestamp префикс (за да не се пипат чужди файлове), сортиране по име
  (zero-padded форматът е лексикографски = хронологично), изтриване на най-старите,
  докато броят стане `keep`.
- Копирането е безопасно без SQLite API, защото се прави **преди** `db.Open` — файлът не е
  отворен от процеса и няма WAL (`sql.Open("sqlite", …)` в
  [internal/db/database.go:19](../../internal/db/database.go#L19) не задава journal pragma).
- Известно ограничение (документира се): два старта в рамките на една и съща секунда дават
  едно и също име → вторият презаписва първия. Приемливо.

### 2. `cmd/movietracker/main.go` — извикване при старт

Между блока с VLC детекцията и `db.Open` ([main.go:74](../../cmd/movietracker/main.go#L74)):

```go
dbPath := filepath.Join(paths.AppFolder, "movietracker.db")

backupPath, err := db.Backup(dbPath, time.Now(), db.BackupKeep)
if err != nil {
    slog.Error("database backup failed", "err", err)
    os.Exit(1)
}
if backupPath != "" {
    slog.Info("database backup created", "path", backupPath)
} else {
    slog.Info("no database to back up (first run)")
}

d, err := db.Open(dbPath)
```

Позицията **преди `db.Open`/`db.Migrate`** е нарочна: снапшотът хваща състоянието отпреди
миграциите, т.е. точно състоянието, към което би се върнал потребителят при проблем.

### 3. Тестове (TDD, red → green) — нов `internal/db/backup_test.go`

Стил като [internal/db/database_test.go](../../internal/db/database_test.go) (`package db_test`,
`t.TempDir()`, `testify/assert`):

| Тест | Проверява |
|------|-----------|
| `TestBackup_CreatesTimestampedCopy` | фиксиран `time.Date(2026, 8, 24, 19, 7, 0, …)` → файл `backup/2026_08_24_19_07_00_movietracker.db` с идентично съдържание; върнатият път сочи този файл |
| `TestBackup_NoSourceFile` | липсваща база → `("", nil)`, без грешка и без създадена `backup/` папка |
| `TestBackup_PrunesOldest` | 12 предварително създадени снапшота + нов → остават точно 10, изтрити са най-старите по timestamp |
| `TestBackup_IgnoresUnrelatedFiles` | `notes.txt` и `2020_01_01_00_00_00_other.db` в `backup/` не се трият и не се броят |
| `TestBackup_RealDatabaseIsUsable` | копие на реална мигрирана база се отваря с `db.Open` + `db.Migrate` и `media` таблицата е налична (integration) |

Без UI промени ⇒ без нови goquery/Playwright тестове; съществуващите остават непокътнати.

### 4. Версия

`Taskfile.yml` → `VERSION: v1.5.0` ⟶ **`v1.6.0`** (нова функционалност, по модела на
итерации 25/33/35).

### 5. Документация (в същия commit)

- **[docs/Открити бъгове.md](../Открити%20бъгове.md)** — секция „Реализирано (Итерация 36)"
  под Bug #21 + `### Статус — ✓ Завършен (Итерация 36)`.
- **[docs/Архитектура.md](../Архитектура.md)**:
  - §3 „Структура на диска (runtime)" — добавя `backup\` към дървото с примерен файл;
  - §6.1 „Стартиране на приложението" — нова стъпка между 4 и 5 („backup на DB файла;
    при неуспех приложението спира");
  - §10 „Лог на решенията" — ново: *„Защо backup преди миграциите и защо retention 10
    (Итерация 36)"* (единственото място, където се обосновава — без дублиране в бъг файла).
- **[README.md](../../README.md)** — обновяване в същия commit (правило от CLAUDE.md).
- Нов **`docs/Итерации/Итерация 36 - Бъг 21 (backup на базата).md`** — планов документ.
- `.gitignore` не се пипа — `*.db` вече покрива снапшотите.

## Верификация

1. `task test` — цялата Go свита зелена, вкл. новите `TestBackup_*`.
2. `task test:e2e` — Playwright свитата зелена (регресия, без промени по нея).
3. `task lint` — чист `gofmt` + `golangci-lint`.
4. **Ръчно, end-to-end:** `task build` → стартиране на `bin/movietracker.exe`; в конзолата
   се вижда `database backup created path=…\backup\<timestamp>_movietracker.db`; файлът
   съществува и е с размера на базата. Второ стартиране → втори файл. Симулация на
   retention: копиране на 12 фиктивни снапшота в `backup\` → след старт остават 10.
5. **Първи старт:** стартиране в празна папка → лог `no database to back up (first run)`,
   без създадена `backup\` папка, приложението работи нормално.

## Дефиниция за готово

- Backup файл с точния формат `YYYY_MM_DD_HH_MM_SS_movietracker.db` в `backup\` при всеки старт
- Пазят се последните 10; по-старите се трият автоматично
- Провал на backup спира стартирането с ясна грешка в лога
- `task test` + `task test:e2e` зелени; версия `v1.6.0`
- Един атомарен commit: планов документ + код + тестове + документация
