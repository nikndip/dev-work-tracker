# Dev Work Tracker

Dev Work Tracker — Telegram-сервис на Go для учёта задач, рабочего времени и стоимости выполненных работ.

Проект находится в активной разработке и реализуется поэтапно. Текущая версия представляет собой базовый инфраструктурный каркас, а не законченную систему учёта.

## О проекте

Dev Work Tracker предназначен для личного учёта рабочих процессов через Telegram. По мере развития система должна поддерживать:

- создание и учёт рабочих задач;
- ручной учёт рабочего времени;
- расчёт стоимости выполненных работ;
- формирование месячных отчётов;
- выгрузку отчётов в Excel;
- учёт оплат.

В текущей опубликованной версии бизнес-функции ещё не реализованы — подготовлена техническая основа для дальнейшей разработки.

## Текущий статус

Проект находится на этапе формирования технической основы.

На текущем этапе реализованы:

- Go-приложение;
- Telegram-бот с long polling;
- команда `/start`;
- централизованное ограничение доступа по Telegram User ID;
- PostgreSQL;
- пул подключений через `pgxpool`;
- Docker и Docker Compose;
- конфигурация через environment variables;
- health endpoint с проверкой PostgreSQL;
- graceful shutdown по `SIGINT` и `SIGTERM`;
- инфраструктура миграций `goose`;
- базовые unit-тесты;
- структурированное JSON-логирование.

## Планируемые возможности

- проекты;
- задачи;
- согласованное плановое время;
- ручной таймер;
- пауза и продолжение работы;
- рабочие сессии;
- история времени;
- ручное исправление рабочего времени;
- расчёт стоимости;
- месячные отчёты;
- экспорт отчётов в `.xlsx`;
- детализация рабочих сессий;
- учёт оплаченных и неоплаченных периодов.

## Технологии

- Go 1.24+
- Echo
- PostgreSQL
- pgx / pgxpool
- goose
- Telegram Bot API (`go-telegram/bot`)
- Docker
- Docker Compose

## Архитектура

```text
Telegram
   │
   ▼
Dev Work Tracker
   ├── Telegram Bot
   ├── HTTP / Health
   └── PostgreSQL
```

Telegram-бот и HTTP-сервер запускаются одним приложением. PostgreSQL подключается через пул соединений, а `/health` проверяет доступность базы данных.

## Структура проекта

```text
dev-work-tracker/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── httpserver/
│   │   ├── server.go
│   │   └── server_test.go
│   ├── storage/
│   │   └── postgres.go
│   └── telegram/
│       ├── bot.go
│       └── bot_test.go
├── migrations/
│   └── 00001_infrastructure.sql
├── .dockerignore
├── .env.example
├── .gitignore
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
└── README.md
```

## Конфигурация

Приложение получает настройки только через environment variables:

| Переменная | Обязательная | Назначение |
|---|---:|---|
| `TELEGRAM_BOT_TOKEN` | да | Токен Telegram-бота |
| `TELEGRAM_ALLOWED_USER_ID` | да | Разрешённый Telegram User ID |
| `DATABASE_URL` | да | Строка подключения PostgreSQL |
| `DEFAULT_TIMEZONE` | да | IANA timezone, например `Europe/Moscow` |
| `DEFAULT_HOURLY_RATE_KOPECKS` | да | Положительная целая ставка в копейках |
| `HTTP_ADDRESS` | нет | Адрес HTTP-сервера, по умолчанию `:8080` |

Для Docker Compose также используются `POSTGRES_DB`, `POSTGRES_USER` и `POSTGRES_PASSWORD`.

Рабочий файл `.env` содержит секреты и локальные настройки. Он исключён из Git и не должен попадать в репозиторий. Для настройки используйте только безопасный шаблон `.env.example`.

## Локальный запуск

1. Клонируйте репозиторий и перейдите в его директорию:

   ```bash
   git clone https://github.com/<owner>/dev-work-tracker.git
   cd dev-work-tracker
   ```

2. Создайте локальный `.env` на основе шаблона:

   ```bash
   cp .env.example .env
   ```

3. Заполните в `.env` локальные значения, включая настоящий Telegram Bot Token, разрешённый Telegram User ID и надёжный пароль PostgreSQL.

4. Запустите приложение и PostgreSQL:

   ```bash
   docker compose up -d --build
   ```

5. Проверьте состояние контейнеров:

   ```bash
   docker compose ps
   ```

6. Проверьте health endpoint:

   ```bash
   curl http://localhost:8080/health
   ```

При доступной базе данных endpoint отвечает статусом `200`:

```json
{"status":"ok"}
```

PostgreSQL намеренно не публикуется наружу через host port.

## Остановка

```bash
docker compose down
```

Данные PostgreSQL хранятся в именованном Docker volume и сохраняются после обычного `docker compose down`. Команда `docker compose down -v` дополнительно удалит этот volume и его данные.

## Миграции PostgreSQL

Для управления миграциями используется `goose`:

```bash
go install github.com/pressly/goose/v3/cmd/goose@v3.24.3
goose -dir migrations postgres "$DATABASE_URL" up
goose -dir migrations postgres "$DATABASE_URL" down
goose -dir migrations postgres "$DATABASE_URL" status
```

`DATABASE_URL` должен указывать на PostgreSQL, доступный из среды, где запускается `goose`. Текущая техническая миграция не создаёт бизнес-таблицы.

## Тесты

```bash
go test ./...
go vet ./...
```

В Dockerfile также предусмотрен отдельный test target:

```bash
docker build --target test -t dev-work-tracker-test .
```

## Telegram

После запуска авторизованный пользователь может открыть [@DevTimeTrackerBot](https://t.me/DevTimeTrackerBot) и отправить:

```text
/start
```

Бот отвечает сообщением о готовности системы. Сообщения и callback-запросы от других Telegram User ID блокируются централизованной проверкой доступа.

## Безопасность

- секреты передаются приложению через environment variables;
- `.env` исключён из Git;
- Telegram-доступ ограничен разрешённым User ID;
- PostgreSQL не публикуется наружу без необходимости;
- реальные токены, пароли, `DATABASE_URL` и другие credentials не должны попадать в репозиторий или логи;
- production-контейнер запускается от непривилегированного пользователя.
