# SAI Storage Service

Микросервис для хранения документов, созданный на Go с использованием SAI framework. Этот сервис предоставляет RESTful API для управления документами в коллекциях с полными CRUD операциями и подключаемыми бэкендами баз данных.

## Возможности

- **Полные CRUD операции**: Создание, чтение, обновление, удаление документов в коллекциях
- **Поддержка подключаемых БД**: Расширяемая архитектура для различных бэкендов баз данных
- **Реализация MongoDB**: Нативный драйвер MongoDB с пулом соединений и оптимизацией (доступна в настоящее время)
- **Гибкая фильтрация**: Поддержка сложных запросов и операций с базой данных
- **Настраиваемость**: Конфигурация на основе переменных окружения с шаблонами
- **Готовность к Docker**: Контейнерное развертывание с Docker Compose
- **Проверки здоровья**: Встроенный мониторинг состояния
- **Документация API**: Автоматическая генерация OpenAPI документации
- **Поддержка middleware**: CORS, логирование, восстановление и аутентификация
- **Валидация**: Валидация запросов с comprehensive обработкой ошибок
- **Управление БД**: Встроенные инструменты для администрирования баз данных

## Быстрый старт

### Предварительные требования

- Go 1.21+ (для локальной разработки)
- Docker и Docker Compose (для контейнерного развертывания)
- База данных (MongoDB поддерживается из коробки, другие могут быть реализованы)

### Локальная разработка

1. **Клонировать репозиторий**
   ```bash
   git clone <repository-url>
   cd sai-storage
   ```

2. **Настроить окружение**
   ```bash
   make setup
   # Отредактировать .env с вашей конфигурацией
   ```

3. **Установить зависимости**
   ```bash
   make deps
   ```

4. **Запустить базу данных (пример MongoDB используя Docker)**
   ```bash
   make up
   ```

5. **Сгенерировать конфигурацию**
   ```bash
   make config
   ```

6. **Запустить сервис**
   ```bash
   make run
   ```

Сервис запустится на `http://localhost:8080` (настраивается через `SERVER_PORT`)

### Развертывание с Docker

1. **Запустить все сервисы с Docker Compose**
   ```bash
   make up
   ```

2. **Просмотреть логи**
   ```bash
   make logs
   ```

3. **Доступ к MongoDB Express (Web UI)**
   ```bash
   make mongo-express
   # Или перейти на http://localhost:8081
   ```

4. **Остановить сервисы**
   ```bash
   make down
   ```

## Эндпоинты API

### Базовый URL
```
http://localhost:8080/api/v1/documents
```

### Создание документов
```http
POST /api/v1/documents/
Content-Type: application/json

{
  "collection": "users",
  "data": [
    {"name": "John", "age": 30, "email": "john@example.com"},
    {"name": "Jane", "age": 25, "email": "jane@example.com"}
  ]
}
```

### Чтение документов
```http
GET /api/v1/documents/
Content-Type: application/json

{
  "collection": "users",
  "filter": {"age": {"$gte": 25}},
  "sort": {"name": 1},
  "limit": 10,
  "skip": 0
}
```

### Обновление документов
```http
PUT /api/v1/documents/
Content-Type: application/json

{
  "collection": "users",
  "filter": {"name": "John"},
  "data": {"$set": {"age": 31, "status": "active"}},
  "upsert": false
}
```

**Операторы обновления:**
- `$set`: Установить значения полей
- `$unset`: Удалить поля
- `$inc`: Увеличить числовые значения
- `$push`: Добавить в массивы

**Примеры:**
```json
// Установить поля
{"data": {"$set": {"age": 31, "email": "john@example.com"}}}

// Удалить поля  
{"data": {"$unset": {"tempField": ""}}}

// Увеличить значение
{"data": {"$inc": {"views": 1}}}
```

### Удаление документов
```http
DELETE /api/v1/documents/
Content-Type: application/json

{
  "collection": "users",
  "filter": {"age": {"$lt": 18}}
}
```

## Конфигурация

Сервис использует переменные окружения для конфигурации. Основные настройки включают:

### Конфигурация сервиса
- `SERVICE_NAME`: Имя сервиса (по умолчанию: `sai-storage`)
- `SERVICE_VERSION`: Версия сервиса (по умолчанию: `1.0.0`)

### Конфигурация сервера
- `SERVER_HOST`: Хост сервера (по умолчанию: `127.0.0.1`)
- `SERVER_PORT`: Порт сервера (по умолчанию: `8080`)
- `SERVER_READ_TIMEOUT`: Таймаут чтения в секундах (по умолчанию: `30`)
- `SERVER_WRITE_TIMEOUT`: Таймаут записи в секундах (по умолчанию: `30`)
- `SERVER_IDLE_TIMEOUT`: Таймаут простоя в секундах (по умолчанию: `120`)

### Конфигурация базы данных
- `DATABASE_TYPE`: Тип базы данных (по умолчанию: `mongo`, расширяемая архитектура для других БД)

### MongoDB-специфичная конфигурация (когда DATABASE_TYPE=mongo)
- `MONGODB_CONNECTION_STRING`: Строка подключения MongoDB
- `MONGO_DATABASE`: Имя базы данных (по умолчанию: `sai`)
- `MONGODB_TIMEOUT`: Таймаут подключения (по умолчанию: `30`)
- `MONGODB_MAX_POOL_SIZE`: Максимальный размер пула (по умолчанию: `100`)
- `MONGODB_MIN_POOL_SIZE`: Минимальный размер пула (по умолчанию: `5`)
- `MONGODB_SELECT_TIMEOUT`: Таймаут выбора сервера (по умолчанию: `5`)
- `MONGODB_IDLE_TIMEOUT`: Таймаут простоя соединения (по умолчанию: `30`)
- `MONGODB_SOCKET_TIMEOUT`: Таймаут сокета (по умолчанию: `30`)
- `MONGO_ROOT_USERNAME`: Имя пользователя администратора MongoDB
- `MONGO_ROOT_PASSWORD`: Пароль администратора MongoDB

### Конфигурация логирования
- `LOG_LEVEL`: Уровень логирования (по умолчанию: `debug`)
- `LOG_OUTPUT`: Вывод логов (по умолчанию: `stdout`)
- `LOG_FORMAT`: Формат логов (по умолчанию: `console`)

## Разработка

### Доступные команды Make

```bash
# Разработка
make deps          # Скачать Go зависимости
make setup         # Создать .env файл из шаблона
make config        # Сгенерировать конфигурацию из шаблона
make config-debug  # Отладка генерации конфигурации
make run           # Запустить приложение локально
make build         # Собрать бинарный файл приложения

# Docker
make docker-build  # Собрать Docker образ
make docker-run    # Запустить Docker контейнер
make up            # Запустить все сервисы с docker-compose
make down          # Остановить все сервисы
make logs          # Показать логи всех сервисов
make logs-app      # Показать логи только приложения
make logs-mongo    # Показать логи только MongoDB
make restart       # Перезапустить все сервисы
make rebuild       # Пересобрать и перезапустить все сервисы

# Управление базой данных
make mongo-shell   # Подключиться к оболочке MongoDB
make mongo-express # Открыть MongoDB Express в браузере
make mongo-reset   # Сбросить данные MongoDB (ВНИМАНИЕ: удаляет все данные!)

# Качество кода
make fmt           # Форматировать Go код
make vet           # Запустить go vet
make lint          # Запустить линтер (требует golangci-lint)
make mod-tidy      # Привести в порядок Go модули

# Тестирование
make test          # Запустить тесты
make test-coverage # Запустить тесты с покрытием

# Мониторинг
make status        # Показать статус всех сервисов
make health        # Проверить здоровье приложения
make version       # Показать информацию о версии

# Очистка
make clean         # Очистить артефакты сборки
make clean-docker  # Очистить Docker ресурсы
make clean-all     # Очистить все

# Инструменты
make check-tools   # Проверить доступность необходимых инструментов
```

### Структура проекта

```
.
├── cmd/                    # Точки входа приложения
│   └── main.go            # Главное приложение
├── internal/              # Внутренние пакеты
│   ├── handlers/          # HTTP обработчики
│   │   └── handler.go
│   ├── mongo/             # Реализация MongoDB
│   │   ├── client.go      # Клиент MongoDB
│   │   └── repository.go  # Репозиторий MongoDB
│   └── service/           # Бизнес-логика
│       └── service.go
├── types/                 # Определения типов
│   ├── request.go         # Типы запросов
│   ├── response.go        # Типы ответов
│   ├── storage.go         # Интерфейсы хранилища
│   └── types.go           # Типы конфигурации
├── scripts/               # Скрипты развертывания
├── config.yaml.template   # Шаблон конфигурации
├── .env                   # Переменные окружения
├── Dockerfile             # Конфигурация Docker
├── docker-compose.yml     # Конфигурация Docker Compose
└── Makefile              # Автоматизация сборки
```

## Проверки здоровья

Сервис включает встроенные проверки здоровья:

```bash
curl http://localhost:8080/health
```

## Документация API

Когда `DOCS_ENABLED=true`, интерактивная документация API доступна по адресу:

```
http://localhost:8080/docs
```

## Аутентификация

Сервис поддерживает два типа аутентификации:

### Аутентификация входящих запросов
Настройка аутентификации для входящих API запросов:

```env
USERNAME=your-username
PASSWORD=your-password
```

### Провайдеры аутентификации
Сервис поддерживает следующие методы аутентификации:

```yaml
auth_providers:
  basic:
    params:
      username: "name"
      password: "pass"
  token:
    params:
      token: "your-token"
```

**Поддерживаемые провайдеры аутентификации:**
- `basic`: HTTP Basic Authentication
- `token`: Token-based authentication

**Переменные окружения для аутентификации:**
```env
AUTH_ENABLED=true
AUTH_PROVIDER=basic
USERNAME=name
PASSWORD=pass
TOKEN=your-token
```

## Управление базой данных

**Примечание: Следующие команды специфичны для MongoDB. Для других реализаций баз данных будут предоставлены эквивалентные команды.**

### MongoDB Express Web UI

Доступ к интерфейсу администрирования MongoDB:

```bash
make mongo-express
# Перейти на http://localhost:8081
```

### Доступ к оболочке MongoDB

Подключение напрямую к MongoDB:

```bash
make mongo-shell
```

### Сброс базы данных

**Внимание: Это удалит все данные!**

```bash
make mongo-reset
```

## Расширение поддержки баз данных

Сервис использует подключаемую архитектуру для бэкендов баз данных. Чтобы добавить поддержку новой базы данных:

1. Реализуйте интерфейс `StorageRepository` из `types/storage.go`
2. Добавьте вашу реализацию в новый пакет (например, `internal/postgres/`)
3. Обновите switch statement в `cmd/main.go` для включения вашего типа БД
4. Добавьте опции конфигурации в `config.yaml.template`

**Текущие реализации:**
- MongoDB (`DATABASE_TYPE=mongo`) - Полная реализация доступна

## Обработка ошибок

Все ответы API следуют единому формату:

**Успешный ответ:**
```json
{
  "data": ["60f1b2b3c8f4b3b3c8f4b3b3"],
  "created": 1
}
```

**Ответ с ошибкой:**
```json
{
  "error": "validation failed",
  "details": "specific error details"
}
```

## Метаданные документов

**Примечание: Поведение метаданных может различаться в зависимости от реализации базы данных.**

Для реализации MongoDB сервис автоматически добавляет метаданные к документам:

- `internal_id`: Уникальный UUID документа
- `cr_time`: Временная метка создания (Unix наносекунды)
- `ch_time`: Временная метка последнего изменения (Unix наносекунды)

## Мониторинг

Сервис включает:

- **Проверки здоровья**: эндпоинт `/health` с проверкой подключения к БД
- **Метрики**: встроенный сбор метрик
- **Логирование**: структурированное логирование с настраиваемыми уровнями
- **Трассировка запросов**: middleware для логирования запросов/ответов
- **Статистика БД**: статистика и мониторинг базы данных (зависит от реализации)

## Возможности производительности

- **Пул соединений**: Настраиваемый пул соединений к базе данных
- **Управление таймаутами**: Настраиваемые таймауты для всех операций
- **Пакетные операции**: Поддержка пакетных вставок и обновлений
- **Индексирование**: Поддержка индексирования БД (зависит от реализации)
- **Оптимизация запросов**: Эффективные паттерны запросов к базе данных

## Вклад в проект

1. Сделайте fork репозитория
2. Создайте ветку для новой функции
3. Внесите изменения
4. Запустите тесты и линтер
5. Отправьте pull request

## Лицензия

Этот проект лицензирован под лицензией MIT - подробности см. в файле LICENSE.

## Поддержка

По вопросам и проблемам создайте issue в репозитории или обратитесь к команде разработки.