# cmd/server

Пример сборки и запуска из командной строки:

```bash
go build -o server && ./server -c="./config.yaml"
```

C флагами версии:

```bash
go build -ldflags "-X main.buildVersion=v1.0.0 -X main.buildDate=$(date -u '+%Y-%m-%d_%H:%M:%S') -X main.buildCommit=$(git rev-parse HEAD)" -o server \
&& ./server -c="./config.yaml"
```
