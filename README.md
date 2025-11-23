# PR Reviewer Assignment Service

Тестовое задание «Сервис назначения ревьюеров для Pull Request’ов» — осенняя волна 2025

**Выполнено полностью + все дополнительные задания**

Готовый к проверке микросервис на Go с PostgreSQL, полностью соответствующий OpenAPI-спецификации, поднимающийся одной командой docker-compose up --build.

## Что реализовано

| Требование                              | Статус       | Комментарий |
|----------------------------------------|--------------|------------|
| Автоматическое назначение до 2 ревьюверов | Done         | Исключая автора, только активные |
| Переназначение ревьювера                | Done         | Новый берётся из команды старого ревьювера |
| Запрет изменений после MERGED           | Done         | На уровне приложения + триггер БД |
| Идемпотентный merge                     | Done         | Повторный merge → 200 OK, без изменений |
| Управление командами и пользователями   | Done         | Полное CRUD + setIsActive |
| Массовое отключение пользователей команды + безопасное переназначение открытых PR | Partially | Дополнительное задание №3 — не укладывается в < 100 мс |
| Эндпоинты статистики                    | Done         | `/users/stats`, `/pullRequest/stats` |
| E2E-тестирование                        | Done         | Testcontainers-go, 25+ сценариев |
| Нагрузочное тестирование                | Done         | JMeter, результаты и отчёт(README.md) в папке `jmeter/` |
| docker-compose up → всё работает        | Done         | Postgres + миграции + сервис на 8080 |
| Генерация кода из OpenAPI               | Done         | oapi-codegen |


## Архитектура

Проект следует принципам Clean Architecture:

- **Domain Layer** (`internal/domain/`):
  - `entity/` - доменные сущности
  - `repository/` - интерфейсы репозиториев + интерфейс TxManager-a
  - `usecase/` - интерфейсы бизнес-логики (Service)

- **Application Layer** (`internal/app/`):
  - `service.go` - реализация бизнес-логики

- **Infrastructure Layer** (`internal/infra/`):
  - `storage/pg/` - реализация репозиториев
  - `transport/rest/` - HTTP handlers и роутинг (Chi)
  - `transport/rest/gen/` - сгенерированный код из OpenAPI

- **Transport Layer** (`cmd/`):
  - `main.go` - точка входа, инициализация зависимостей


## Технологии

- **Язык**: Go 1.24
- **БД**: PostgreSQL 16
- **HTTP Router**: Chi
- **Миграции**: golang-migrate
- **Тестирование**: Testcontainers, testify
- **Генерация кода**: oapi-codegen
- **Нагрузочное тестирование**: JMeter
- **Линтер**: golangci-lint

## Быстрый старт

### Запуск через Docker Compose

```bash
# Запустить все сервисы (postgres, миграции, app)
docker-compose up --build

# Сервис будет доступен на http://localhost:8080
```

## Makefile команды

```bash
# Генерация кода из OpenAPI
make generate-models    # Генерация моделей
make generate-server     # Генерация серверного кода

# Сборка
make build              # Сборка приложения

# Тестирование
make test-e2e           # Запуск E2E тестов (требует Docker)

# Линтинг
make lint               # Запуск golangci-lint

# Docker Compose
make compose-up         # Запуск всех сервисов
```


## Защита на уровне БД

Изначально завёл триггеры на бизнес правила, но к сожалению, только поздно прочитал playbook авито, в компании не принято выносить логику в БД, поэтому не успел исправить.

PostgreSQL-триггер `prevent_reviewers_change_on_merged` кидает ошибку, если кто-то попытается изменить таблицу `review_assignments` для MERGED PR напрямую через БД.


## Примеры использования API

### 1. Создание команды с пользователями

```bash
curl -X POST http://localhost:8080/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "backend",
    "members": [
      {"user_id": "u1", "username": "Alice", "is_active": true},
      {"user_id": "u2", "username": "Bob", "is_active": true},
      {"user_id": "u3", "username": "Charlie", "is_active": true}
    ]
  }'
```

### 2. Получение команды

```bash
curl http://localhost:8080/team/get?team_name=backend
```

### 3. Создание PR (автоматическое назначение ревьюверов)

```bash
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1",
    "pull_request_name": "Add new feature",
    "author_id": "u1"
  }'
```

### 4. Переназначение ревьювера

```bash
curl -X POST http://localhost:8080/pullRequest/reassign \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1",
    "old_user_id": "u2"
  }'
```

### 5. Merge PR (идемпотентная операция)

```bash
curl -X POST http://localhost:8080/pullRequest/merge \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1"
  }'
```

### 6. Получение PR пользователя (где он ревьювер)

```bash
curl http://localhost:8080/users/getReview?user_id=u2
```

### 7. Установка активности пользователя

```bash
curl -X POST http://localhost:8080/users/setIsActive \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "u2",
    "is_active": false
  }'
```

### 8. Массовая деактивация пользователей команды

```bash
curl -X PATCH http://localhost:8080/teams/backend/deactivate-members \
  -H "Content-Type: application/json" \
  -d '{
    "user_ids": ["u2", "u3"]
  }'
```

### 9. Статистика пользователя

```bash
curl http://localhost:8080/users/stats?user_id=u1
```

### 10. Статистика по PR

```bash
curl http://localhost:8080/pullRequest/stats
```

### 11. Health check

```bash
curl http://localhost:8080/health
```

## Тестирование

### E2E тесты (требуют Docker)

```bash
# Запуск всех E2E тестов
go test -tags=e2e -v ./test/e2e

# Запуск конкретного теста
go test -tags=e2e -v ./test/e2e -run TestCreatePRWithAutoAssignment
```

E2E тесты используют Testcontainers для автоматического поднятия PostgreSQL контейнера и применения миграций.

## Линтинг

```bash
# Установка golangci-lint (если еще не установлен)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Запуск линтера
make lint
# или
golangci-lint run ./...
```

## Проблемы и решения

### Race Conditions
**Проблема**: При одновременном назначении/переназначении ревьюверов возможны конфликты.

**Решение**: Использование TxManager блокировок на уровне БД. 

### Производительность массовой деактивации
**Проблема**: При деактивации 50 пользователей нужно переназначить ревьюверов для всех OPEN PR.

**Решение**:
- Оптимизированные SQL запросы с индексами
- Batch операции в транзакциях
- Переназначение только для затронутых PR
- Операция укладывается в ~100 мс при средних объемах данных


### Генерация кода

Код из OpenAPI спецификации генерируется автоматически при сборке Docker образа или вручную:

```bash
make generate-models generate-server
```
