# Итерация 09: VLC detection

## Цел

Открий VLC изпълнимия файл при старт на приложението: първо провери стандартните Windows пътища, после прочети override от `%LOCALAPPDATA%\MovieTracker\config.toml`. Ако не е намерен — логни предупреждение (UI съобщение идва в Итерация 10).

## Архитектура

### `internal/config/` — per-computer TOML конфиг

```go
type Config struct {
    VLCPath string `toml:"vlc_path"`
}
func Load() (*Config, error)
```

- Чете от `%LOCALAPPDATA%\MovieTracker\config.toml`
- Ако файлът липсва → `&Config{}, nil` (не е грешка)
- Ако файлът съществува но е невалиден → грешка

### `internal/vlc/` — VLC path detection

```go
var DefaultPaths = []string{
    `C:\Program Files\VideoLAN\VLC\vlc.exe`,
    `C:\Program Files (x86)\VideoLAN\VLC\vlc.exe`,
}
var ErrNotFound = errors.New("VLC not found; set vlc_path in config.toml")

func Detect(candidates []string, configOverride string) (string, error)
func DetectDefault(configOverride string) (string, error)
```

`Detect` приема инжектируем списък пътища (за testability). `DetectDefault` ползва `DefaultPaths`.

**Логика:**
1. Итерира `candidates` — при намерен файл (`os.Stat` успешен) → връща пътя
2. Ако никой от `candidates` не съществува → опитва `configOverride` (ако е непразен и съществува)
3. Ако нищо не е намерено → `"", ErrNotFound`

### Wire up в `main.go`

```go
cfg, err := config.Load()
if err != nil {
    log.Fatalf("config load failed: %v", err)
}
vlcPath, err := vlc.DetectDefault(cfg.VLCPath)
if err != nil {
    log.Printf("warning: %v", err)
} else {
    log.Printf("VLC found: %s", vlcPath)
}
```

VLC пътят се логва, но не се ползва за нищо друго — захранва се в Итерация 10.

## TDD тестове

### `internal/vlc/vlc_test.go`

| Тест | Проверява |
|------|-----------|
| `TestDetect_ReturnsFirstMatchingPath` | От списък с 2 пътища, вторият съществува → връща него |
| `TestDetect_UsesConfigOverride_WhenNoStandardFound` | Standard пътища липсват, config override съществува → override |
| `TestDetect_ReturnsError_WhenNothingFound` | Нищо не съществува → `ErrNotFound` |
| `TestDetect_IgnoresAbsentConfigOverride` | Config override пътят не съществува → `ErrNotFound` |

Тестовете създават реални temp файлове (чрез `t.TempDir()`) и ги подават като `candidates`.

### `internal/config/config_test.go`

| Тест | Проверява |
|------|-----------|
| `TestLoad_ReturnsDefaultConfig_WhenFileAbsent` | Без файл → `&Config{}`, без грешка |
| `TestLoad_ParsesVLCPath` | TOML с `vlc_path = "C:\\path\\vlc.exe"` → `Config.VLCPath` е попълнен |

Тестовете подменят `LOCALAPPDATA` env var чрез `t.Setenv` за изолация.

## Реализирани файлове

- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/vlc/vlc.go`
- `internal/vlc/vlc_test.go`
- `cmd/movietracker/main.go` (добавени Load + DetectDefault)
