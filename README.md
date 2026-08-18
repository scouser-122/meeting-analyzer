# meeting-analyzer

Умный помощник для конспектирования встреч

## Тех. стек

Язык программирования: Go (версия 1.26.4)

Основные фреймворки и библиотеки:
- HTTP сервер: Стандартная библиотека `net/http` + `http.ServeMux`
- Конфигурация: `github.com/caarlos0/env/v6 v6.10.1` (переменные окружения) + `gopkg.in/yaml.v3 v3.0.1` (YAML-файлы)
- База данных: PostgreSQL с драйвером `github.com/jackc/pgx/v5 v5.10.0` (включая pgxpool, pgconn, pgerrcode). Используются SQL-миграции.
- Миграции:`github.com/golang-migrate/migrate/v4 v4.19.1`
- HTTP-клиенты для внешних API: `github.com/go-resty/resty/v2 v2.17.2` (GigaChat, SaluteSpeech)
- Аутентификация: `github.com/golang-jwt/jwt/v4 v4.5.2`
- TUI (терминальный интерфейс): `github.com/charmbracelet/bubbletea v1.3.10`, `github.com/charmbracelet/bubbles v1.0.0`, `github.com/charmbracelet/lipgloss v1.1.0` и связанные пакеты (ansi, termenv и др.)
- Логирование: `log/slog` (стандарт) + `github.com/alchemy/rotoslog v1.0.1` (ротация файлов) + `github.com/hydraide/hydraide v1.0.0` (мульти-хендлеры)
- Утилиты: `github.com/google/uuid v1.6.0`, встроенные пакеты `sync`, `context`, `encoding/json`, `time`, `os`, `filepath`

Внешние сервисы:
- LLM / суммаризация: GigaChat API (Sber)
- Транскрипция аудио: SaluteSpeech API (Сбер)

Дополнительно: 
- Собственный static linter (osexitanalizer + ineffassign, honnef.co/go/tools), 
- worker для асинхронной обработки встреч, 
- domain-driven структура (internal/domain, service, repository, server, tui, worker).


## Cтруктура проекта
```
meeting-analyzer/
├── cmd/                          # Точки входа (main.go для каждого интерфейса)
│   ├── server/                   # Основной backend-сервер (HTTP API)
│   ├── tui/                      # TUI-утилита (CLI-интерфейс)
│   ├── mocks/                    # Заглушки для интеграций с обработчиком аудио и LLM
│   ├── staticlint/               # Checker для статического анализа
│   └── telegram-bot/             # Telegram-бот
│
├── internal/                     # Внутренние пакеты (не экспортируются наружу)
│   ├── client                    # Клиенты для интеграции с обработчиком аудио и LLM
│   ├── config                    # Конфигурация приложения (загрузка из env/файла)
│   ├── domain                    # Модели и репозитории для работы с данными
│   ├── logger                    # Логирование
│   ├── models                    # Модели данных для взаимодействия между модулями
│   ├── repository                # Реализация репозиториев для работы с данными
│   ├── server                    # Обработка входящих запросов в сервисе бэкенда
│   ├── service                   # Сервисный слой бэкенда
│   ├── storage                   # Реализация файлового хранилища
│   ├── telegram                  # Реализация Telegram бота
│   ├── tui                       # Реализация TUI утилиты
│   └── worker                    # Обработчики асинхронных задач
│
├── migrations/                   # SQL-миграции для PostgreSQL
├── go.mod
├── go.sum
└── README.md
```

## Команды

### Запуск линтера 
```bash
go vet -vettool=./cmd/staticlint/multichecker ./...
```

### Запуск интеграционных и юнит тестов 
```bash
go test ./...
```

### Запуск теста на race condition
```bash
go test -race ./internal/worker/...
```

### Скрипт для отката миграций БД
```bash
migrate -database "postgres://postgres:123@localhost:5432/meeting_analyzer?sslmode=disable" -path ./migrations down
```

### Запуск локального сервера minio
```bash
minio server ./cmd/server/data/minio
```