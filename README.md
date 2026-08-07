# meeting-analyzer — структура проекта

```
meeting-analyzer/
├── cmd/                          # Точки входа (main.go для каждого интерфейса)
│   ├── server/                   # Основной backend-сервер (HTTP/gRPC API)
│   ├── tui/                      # TUI-утилита (CLI-интерфейс)
│   ├── mocks/                    # Заглушки для интеграций с обработчиком аудио и LLM
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
│   ├── tui                       # Реализация TUI утилиты
│   └── worker                    # Обработчики асинхронных задач
│
├── migrations/                   # SQL-миграции для PostgreSQL
├── go.mod
├── go.sum
└── README.md
```

## DB migrations down

```bash
migrate -database "postgres://postgres:123@localhost:5432/meeting_analyzer?sslmode=disable" -path ./migrations down
```