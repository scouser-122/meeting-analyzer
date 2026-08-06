# meeting-analyzer — структура проекта

```
meeting-analyzer/
├── cmd/                          # Точки входа (main.go для каждого интерфейса)
│   ├── server/                   # Основной backend-сервер (HTTP/gRPC API)
│   ├── tui/                      # TUI-утилита (CLI-интерфейс)
│   ├── vk-bot/                   # VK-бот
│   └── telegram-bot/             # Telegram-бот
│
├── internal/                     # Внутренние пакеты (не экспортируются наружу)
│   ├── app/                      # Слой приложения
│   │   ├── worker/               # Фоновая обработка задач — асинхронная транскрипция,
│   │   |                         #   получение краткой выжимки, обновление статусов
│   │   └── service/              # Бизнес-логика (use cases) — загрузка встречи,
│   │                             #   обработка, поиск, вопросы к GigaChat
│   │
│   ├── domain/                   # Доменный слой
│   │   ├── model/                # Модели данных: User, Meeting, Transcription, Task
│   │   └── repository/           # Интерфейсы (порты) репозиториев для доступа к данным
│   │
│   ├── repository/               # Слой работы с БД
│   │   └── postgres/             # Реализация репозиториев для PostgreSQL
│   │
│   ├── client/                   # Клиенты внешних API
│   │   ├── salutespeech/         # Клиент SaluteSpeech API (интерфейс + реализация)
│   │   └── gigachat/             # Клиент GigaChat API (интерфейс + реализация)
│   │
│   └── config/                   # Конфигурация приложения (загрузка из env/файла)
│
├── migrations/                   # SQL-миграции для PostgreSQL
├── go.mod
├── go.sum
└── README.md
```

## Назначение каталогов

| Каталог | Назначение |
|---|---|
| `cmd/server/` | Точка входа основного backend-сервера. Инициализирует зависимости, поднимает HTTP/gRPC API |
| `cmd/tui/` | Точка входа TUI-утилиты. Тонкий адаптер — принимает команды пользователя и передаёт в `internal/app/command/` |
| `cmd/vk-bot/` | Точка входа VK-бота. Принимает сообщения из VK, преобразует в команды и передаёт в слой `command` |
| `cmd/telegram-bot/` | Точка входа Telegram-бота. Аналогично VK-боту — адаптер без бизнес-логики |
| `internal/app/command/` | Слой обработки команд. Получает DTO от UI-адаптеров, валидирует, вызывает сервисы бизнес-логики |
| `internal/app/service/` | Слой бизнес-логики. Содержит всю основную логику: регистрация пользователя, загрузка встречи, создание задачи, поиск, вопросы к GigaChat. Не зависит от UI и БД |
| `internal/domain/model/` | Доменные модели: `User`, `Meeting`, `Transcription`, `Task`. Чистые структуры без зависимостей |
| `internal/domain/repository/` | Интерфейсы репозиториев (порты): `UserRepository`, `MeetingRepository`, `TaskRepository`. Определяют контракт доступа к данным |
| `internal/repository/postgres/` | Реализация интерфейсов репозиториев для PostgreSQL (драйвер pgx или database/sql) |
| `internal/client/salutespeech/` | Клиент SaluteSpeech API. Содержит интерфейс (для подмены на заглушку в тестах) и реальную реализацию |
| `internal/client/gigachat/` | Клиент GigaChat API. Аналогично — интерфейс + реализация |
| `internal/worker/` | Фоновая обработка задач: принимает задачи из очереди, вызывает SaluteSpeech для транскрипции, затем GigaChat для выжимки, обновляет статусы в БД |
| `internal/config/` | Структуры конфигурации и их загрузка (из переменных окружения / YAML-файла) |
| `migrations/` | SQL-файлы миграций для создания и изменения схемы PostgreSQL |

## Соответствие архитектурным требованиям

- **UI-слой** (`cmd/tui/`, `cmd/vk-bot/`, `cmd/telegram-bot/`) — тонкие адаптеры, не содержат бизнес-логики
- **Слой обработки команд** (`internal/app/command/`) — принимает команды, валидирует, делегирует сервисам
- **Слой бизнес-логики** (`internal/app/service/`) — независим от UI и способа взаимодействия
- **Слой работы с БД** (`internal/repository/postgres/`) — изолирован за интерфейсами из `domain/repository/`
- **Клиенты внешних API** (`internal/client/`) — каждый клиент имеет интерфейс, что позволяет подменить реальную реализацию тестовой заглушкой
- **Фоновая обработка** (`internal/worker/`) — асинхронная обработка задач, отделена от синхронных запросов

## DB migrations down

```bash
migrate -database "postgres://postgres:123@localhost:5432/meeting_analyzer?sslmode=disable" -path ./migrations down
```