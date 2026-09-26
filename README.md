# Dev Work Tracker

Dev Work Tracker — production-ready Telegram-бот на Go для ручного учёта выполненных работ, рабочего времени, стоимости и оплат. Проект продолжает развиваться, но текущая версия уже содержит законченный пользовательский MVP.

## Возможности

- проекты с индивидуальной почасовой ставкой и деактивацией;
- ручное добавление работы через пошаговый Telegram-интерфейс и предпросмотр;
- логическая дата работы с расчётом «сегодня» и «вчера» в `Europe/Moscow`;
- гибкий ввод длительности: `90 мин`, `1 час 30 мин`, `1:30`, `1,5 часа` и другие формы;
- точный расчёт денег целочисленной арифметикой в копейках;
- снимок ставки в каждой записи: изменение ставки проекта не меняет старые работы;
- просмотр, редактирование и удаление работ;
- месячные отчёты с разбивкой по проектам;
- профессиональный экспорт `.xlsx` за месяц по всем или одному проекту;
- статусы оплаты `pending` / `paid` для пары проект + месяц;
- централизованный Telegram allowlist;
- PostgreSQL, health endpoint, graceful shutdown и retry подключения к БД.

Основной интерфейс работает через inline-кнопки. Также доступны команды `/start`, `/new`, `/report`, `/export`, `/projects`, `/payments` и `/cancel`.

## Технологии

- Go 1.24+
- PostgreSQL 17, pgx / pgxpool
- Telegram Bot API (`go-telegram/bot`)
- Excelize
- Echo
- goose migrations
- Docker и Docker Compose

## Структура

```text
cmd/server/              запуск приложения
internal/config/         environment-конфигурация
internal/domain/         бизнес-модели
internal/duration/       разбор и форматирование длительности
internal/export/         генерация XLSX
internal/httpserver/     GET /health
internal/money/          точная денежная математика
internal/repository/     PostgreSQL-запросы
internal/service/        бизнес-правила
internal/storage/        подключение к PostgreSQL
internal/telegram/       меню и пользовательские сценарии
migrations/              goose migrations
```

## Данные

Бизнес-данные хранятся в трёх таблицах:

- `projects`: название, текущая ставка в копейках, признак активности;
- `work_entries`: проект, дата, описание, длительность, снимок ставки и рассчитанная сумма;
- `payment_periods`: статус оплаты проекта за календарный месяц.

Сумма записи вычисляется на backend без `float64`:

```text
(hourly_rate_kopecks * duration_seconds + 1800) / 3600
```

Это округление положительного результата до ближайшей копейки. Итог отчёта всегда является суммой сохранённых `amount_kopecks`.

## Конфигурация

Приложение читает только environment variables:

| Переменная | Обязательная | Назначение |
|---|---:|---|
| `TELEGRAM_BOT_TOKEN` | да | токен Telegram-бота |
| `TELEGRAM_ALLOWED_USER_ID` | да | единственный разрешённый Telegram User ID |
| `DATABASE_URL` | да | PostgreSQL connection string |
| `DEFAULT_TIMEZONE` | да | IANA timezone, например `Europe/Moscow` |
| `DEFAULT_HOURLY_RATE_KOPECKS` | да | ставка нового проекта по умолчанию в копейках |
| `HTTP_ADDRESS` | нет | HTTP address, по умолчанию `:8080` |

Docker Compose дополнительно использует `POSTGRES_DB`, `POSTGRES_USER` и `POSTGRES_PASSWORD`. Создайте `.env` из `.env.example`; рабочий `.env` исключён из Git.

## Запуск

```bash
docker compose up -d postgres
goose -dir migrations postgres "$DATABASE_URL" up
docker compose up -d --build
docker compose ps
curl http://localhost:8080/health
```

PostgreSQL не публикуется на host. Поэтому при Docker-запуске migration runner должен находиться в compose network и использовать hostname `postgres`, либо миграции следует выполнять из доверенной среды с доступом к базе.

Остановка без удаления данных:

```bash
docker compose down
```

Не используйте `docker compose down -v`, если volume с данными нужно сохранить.

## Миграции

Проект использует goose v3.24.3:

```bash
go install github.com/pressly/goose/v3/cmd/goose@v3.24.3
goose -dir migrations postgres "$DATABASE_URL" status
goose -dir migrations postgres "$DATABASE_URL" up
```

Миграция `00002_work_tracking_mvp.sql` создаёт `projects`, `work_entries`, `payment_periods`, внешние ключи, ограничения и индексы для запросов по проекту и периоду.

## Проверки

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
docker compose config
docker build --target test -t dev-work-tracker-test .
```

Тесты покрывают денежное округление, разбор длительности, снимок ставки, перерасчёт суммы после изменения времени, границы месячного отчёта и структуру Excel-файла.

## Безопасность

- сообщения и callback-запросы доступны только `TELEGRAM_ALLOWED_USER_ID`;
- `.env`, токены, пароли и полный `DATABASE_URL` не коммитятся и не логируются;
- PostgreSQL не имеет внешнего порта;
- HTTP доступен только через binding, заданный в Compose;
- production-контейнер работает от непривилегированного пользователя.
