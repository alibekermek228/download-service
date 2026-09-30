# Сервис асинхронного скачивания файлов

Учебный проект для производственной практики. Сервис принимает набор URL, сразу создаёт заявку и скачивает файлы в фоне. Состояние процесса хранится в Temporal, а заявки и содержимое файлов - в PostgreSQL.

## Что реализовано

- `POST /downloads` создаёт заявку со статусом `PROCESS`.
- `GET /downloads/{id}` возвращает статус и результаты каждого файла.
- `GET /downloads/{id}/files/{file_id}` отдаёт сохранённые байты.
- Файлы скачиваются параллельно с ограничением количества goroutine.
- Один общий timeout применяется ко всей заявке.
- Ошибка одного URL не останавливает остальные загрузки.
- Temporal выполняет фоновый процесс и повторяет инфраструктурные ошибки.
- PostgreSQL хранит заявки, ошибки и файлы в поле `BYTEA`.
- Есть middleware для `Request ID` и восстановления после panic.
- Реализован graceful shutdown продолжительностью не более минуты.
- Основные use case, HTTP-обработчики и загрузчик покрыты unit-тестами.

## Связь с учебными материалами

В проекте намеренно используются только основные конструкции из курса:

- структуры и интерфейсы - модели, репозитории и зависимости use case;
- `errors.Is`, `errors.New` и `fmt.Errorf` - обработка ошибок;
- `context` и timeout - остановка HTTP-запросов и goroutine;
- goroutine, channels, `sync.WaitGroup` и `sync.Mutex` - параллельная загрузка;
- `net/http` и `encoding/json` - HTTP API;
- пакет `testing` и ручные моки - unit-тесты;
- слои domain, usecase, adapters и transport - чистая архитектура;
- workflow и activities - фоновая работа через Temporal.

## Структура проекта

```text
cmd/service                 запуск приложения
internal/domain             доменные модели и ошибки
internal/usecase            бизнес-логика
internal/transport/http     HTTP-запросы, ответы и middleware
internal/repository/postgres работа с PostgreSQL
internal/downloader         скачивание по HTTP
internal/temporalapp        Temporal workflow и activities
migrations                  схема базы данных
```

## Запуск

Понадобится Docker Desktop с поддержкой `docker compose`.

```bash
docker compose up --build
```

После запуска доступны:

- API: `http://localhost:8081`;
- Temporal UI: `http://localhost:8080`;
- PostgreSQL приложения: `localhost:5432`.

Остановка:

```bash
docker compose down
```

Чтобы удалить тестовые данные вместе с Docker volumes:

```bash
docker compose down -v
```

## Пример работы

Создание заявки:

```bash
curl -X POST http://localhost:8081/downloads \
  -H "Content-Type: application/json" \
  -d '{
    "files": [
      {"url": "https://example.com"},
      {"url": "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf"}
    ],
    "timeout": "60s"
  }'
```

Пример ответа:

```json
{
  "id": 1,
  "status": "PROCESS"
}
```

Проверка результата:

```bash
curl http://localhost:8081/downloads/1
```

Скачивание сохранённого файла:

```bash
curl http://localhost:8081/downloads/1/files/1 --output result.bin
```

## Тесты

```bash
go test ./...
go test -race ./...
```

## Принятые ограничения

- в одной заявке допускается от 1 до 20 файлов;
- максимальный timeout заявки - 5 минут;
- максимальный размер одного файла - 10 МБ;
- одновременно скачиваются не более 5 файлов;
- принимаются только абсолютные URL со схемой `http` или `https`;
- OpenAPI относится к части повышенной сложности и пока не добавлен.

Значения размера файла и количества параллельных загрузок можно изменить переменными окружения из `.env.example`.
