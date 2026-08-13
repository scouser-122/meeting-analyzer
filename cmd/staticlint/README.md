# Запуск статических тестов

Cборка:
```bash
go build -o multichecker
```

Выполнить в корне:
```bash
go vet -vettool=./cmd/staticlint/multichecker ./...
```
