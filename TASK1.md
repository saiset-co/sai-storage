# ТЗ: Админ-панель sai-storage

sai-storage -
Делаем админку, пример sai-constrol/internal/admin.go

1. Список коллекций (вес, количество документов, количество индексов, выполнить запрос или новый ссохранением, либо из сохраненного списка)
2. Список индексов, с возможностью добавить индекс в коллекцию 
3. Список частых запросов, на основе каждого запроса возможность создать индекс, если его нет
4. Список долгих запросов, на основе каждого запроса возможность создать индекс, если его нет
5. Список логов на обновление записей в БД, (по параметру из конфига мы логируем данные перед обновлением), добавить возможность сделать откат изменения всей цепочки запросов)
6. Список логов на удаления записей в БД, (по параметру из конфига мы логируем данные перед удалением), добавить возможность сделать откат удаления всей цепочки запросов)
7. Список кастомных запросов, свозможностью добавить новый

Только реальные данные, никаких мокапов и предположений!

## План реализации

### Этап 0 — Обновление sai-service до v1.1.9 ✅
`go get github.com/saiset-co/sai-service@v1.1.9 && go mod tidy`

### Этап 1 — Новые типы (`types/admin.go`) ✅
`CollectionStats`, `IndexInfo`, `SlowQuery`, `QueryStat`, `CustomQuery`, `CreateIndexRequest`, `RestoreRequest`.
Расширить `StorageFeaturesConfig`: `+TrackQueryStats`, `+SlowQueryThresholdMs`.

### Этап 2 — Расширение `StorageRepository` interface ✅
6 новых методов: `GetAdminCollectionStats`, `ListCollectionNames`, `ListIndexes`, `CreateIndex`, `GetSlowQueries`, `SetSlowQueryThreshold`.

### Этап 3 — MongoDB реализация (`internal/mongo/admin.go`) ✅
`collStats` через `RunCommand`, `Indexes().List()`, `Indexes().CreateOne()`, `system.profile`.

### Этап 4 — Redis заглушки (`internal/redis/admin.go`) ✅
Пустые реализации, возвращающие `nil` / пустые слайсы.

### Этап 5 — `archive_operation_id` в `service.go` + `trackQueryStats` ✅
В `writeArchive`: `archive_operation_id` (UUID), `archive_time`, `source_collection`.
В CRUD-методах: upsert в `_admin_query_stats` при `TrackQueryStats=true`.

### Этап 6 — API endpoints (`internal/handlers/admin_handler.go`) ✅
```
POST /api/v1/admin/indexes
POST /api/v1/admin/restore/update
POST /api/v1/admin/restore/delete
POST /api/v1/admin/slow-queries/threshold
```

### Этап 7 — Admin панель ✅
- `internal/admin.go` — `SetupAdmin()`, регистрация 7 страниц
- `internal/admin_stats.go` — страницы 1 (коллекции) и 2 (индексы)
- `internal/admin_queries.go` — страницы 3 (частые), 4 (долгие), 7 (кастомные)
- `internal/admin_restore.go` — страницы 5 (логи UPDATE) и 6 (логи DELETE)

### Этап 6 — API endpoints (`internal/handlers/admin_handler.go`) ✅
```
POST /admin/indexes               — CreateIndexFromForm
POST /admin/restore/update        — RestoreUpdate
POST /admin/restore/delete        — RestoreDelete
POST /admin/slow-queries/threshold — SetSlowQueryThreshold
POST /admin/custom-queries        — SaveCustomQuery
POST /admin/custom-queries/delete — DeleteCustomQuery
```

### Этап 8 — Обновление `cmd/main.go` ✅
Вызов `internal.SetupAdmin(storageService, handler)`.

### Этап 9 — Конфиг ✅
`config.template.yml` и `.env.example`: `+STORAGE_TRACK_QUERY_STATS`, `+STORAGE_SLOW_QUERY_THRESHOLD_MS`.

---

## Анализ текущего состояния

### Что уже реализовано

**В `internal/service/service.go`:**
- `ArchiveChanges` — перед UPDATE/DELETE копирует документы в `{collection}_update_archive` и `{collection}_delete_archive`. Поля архива: все поля оригинального документа + `cr_time`/`ch_time` (момент архивации). `_id` удаляется.
- `LogRequests` — логирует каждый входящий HTTP-запрос в `{collection}_request_logs` с полями: `collection`, `method`, `path`, `query`, `body`, `request_time`, `request_unix`, `ip`, `user`.

**В `internal/mongo/client.go`:**
- `ListCollectionNames(ctx)` — список имён коллекций
- `CreateIndexes(ctx, collectionName, []IndexModel)` — создание индексов
- `GetStats(ctx)` — `dbStats` (статистика всей БД, не коллекции)

**Во фреймворке `sai-service/admin`:**
- `Builder` с поддержкой `Page`, `Resource`, AJAX-форм через `data-admin-ajax="true"`, flash-сообщений, фрагментного обновления контента (`X-SAI-Admin-Fragment`).
- Actions: `WriteActionJSON(ctx, message, err)` для POST-ответов.

### Что отсутствует и нужно добавить

---

## Раздел 1: Список коллекций

### Что нужно в mongo.Client
Новый метод `GetCollectionStats(ctx, collectionName string) (CollectionStats, error)`:
```go
type CollectionStats struct {
    Name        string
    Count       int64
    StorageSize int64  // байты
    IndexSize   int64  // байты
    NumIndexes  int
}
```
Реализация через `db.RunCommand(ctx, bson.M{"collStats": collectionName})`. Поля из результата: `count`, `storageSize`, `totalIndexSize`, `nindexes`.

### Что нужно в StorageRepository interface
Добавить метод `GetAdminStats(ctx) (AdminStats, error)` — возвращает список `CollectionStats` по всем коллекциям, исключая служебные (`_admin_*`, `*_update_archive`, `*_delete_archive`, `*_request_logs`).

### Функциональность UI
- Таблица: имя коллекции, кол-во документов, размер данных, размер индексов, кол-во индексов.
- Кнопка "Выполнить запрос" → переход на страницу выполнения запросов для этой коллекции.
- Кнопка "Из сохранённого" → выбор из `_admin_saved_queries` для этой коллекции.

---

## Раздел 2: Список индексов

### Что нужно в mongo.Client
Новый метод `ListIndexes(ctx, collectionName string) ([]IndexInfo, error)`:
```go
type IndexInfo struct {
    Name   string
    Fields map[string]int  // field → direction (1 asc, -1 desc)
    Unique bool
    Sparse bool
}
```
Реализация через `collection.Indexes().List(ctx)` → `cursor.All(ctx, &results)`.

### Новый endpoint для создания индекса
`POST /api/v1/admin/indexes` — принимает:
```go
type CreateIndexRequest struct {
    Collection string         `json:"collection"`
    Keys       map[string]int `json:"keys"`     // {"field": 1, "field2": -1}
    Unique     bool           `json:"unique"`
    Sparse     bool           `json:"sparse"`
    Name       string         `json:"name"`
}
```
Только для admin-группы (basic auth).

### Функциональность UI
- Выбор коллекции → список индексов с полями.
- Форма добавления индекса: коллекция, поля (динамически добавляемые пары field+direction), unique, sparse.

---

## Раздел 3: Частые запросы

### Проблема
Текущий `LogRequests` пишет каждый запрос целиком в `{collection}_request_logs`. Нет агрегации по паттернам фильтров.

### Что нужно добавить
**Опция конфига `StorageFeaturesConfig`:**
```go
type StorageFeaturesConfig struct {
    LogRequests     bool `yaml:"log_requests"`
    ArchiveChanges  bool `yaml:"archive_changes"`
    TrackQueryStats bool `yaml:"track_query_stats"`   // новое
}
```

**Логика:** при каждом запросе (если `track_query_stats: true`) инкрементировать счётчик в `_admin_query_stats`:
```
{
  collection: "users",
  operation:  "read",  // create/read/update/delete/aggregate
  filter_fingerprint: MD5(sorted JSON ключей фильтра),  // только ключи, без значений
  filter_keys: ["status", "role"],
  count: 47,
  last_seen: <unix_nano>
}
```
Операция: upsert по `(collection, operation, filter_fingerprint)` + `$inc: {count: 1}`.

### Функциональность UI
- Таблица: коллекция, операция, поля фильтра, количество вызовов, последний вызов.
- Сортировка по count desc.
- Для каждой строки: кнопка "Создать индекс" → открывает форму с предзаполненными полями из `filter_keys`.
- Перед созданием: проверить через `ListIndexes` нет ли уже индекса по этим полям.

---

## Раздел 4: Долгие запросы

### MongoDB
**Включение MongoDB Profiler** через `db.RunCommand(ctx, bson.M{"profile": 1, "slowms": threshold})`.

Новые методы в mongo.Client:
```go
func (c *Client) EnableProfiler(ctx context.Context, slowMs int) error
func (c *Client) GetSlowQueries(ctx context.Context, limit int) ([]SlowQuery, error)
```
```go
type SlowQuery struct {
    Op           string    // "query", "update", "insert", "remove", "command"
    Namespace    string    // "db.collection"
    Collection   string
    DurationMs   int64
    KeysExamined int64
    DocsExamined int64
    PlanSummary  string    // "COLLSCAN" или "IXSCAN { field: 1 }"
    FilterKeys   []string  // извлечённые ключи из command.filter
    Timestamp    time.Time
}
```
Данные из коллекции `system.profile`. Требует прав `dbAdmin` или `clusterAdmin`.

**Опция конфига:**
```yaml
storage:
  features:
    slow_query_threshold_ms: 100   # 0 = отключено
```

### CloverDB
CloverDB не имеет профайлера. Нужно инструментировать в коде.

**Что добавить:** в CloverDB-реализации `StorageRepository` обернуть каждый вызов замером времени:
```go
start := time.Now()
result, err := c.db.Query(...).FindAll()
elapsed := time.Since(start)
if elapsed >= threshold {
    // сохранить в _admin_slow_queries
}
```
Структура записи аналогична `SlowQuery` выше, без `KeysExamined`/`DocsExamined` (CloverDB не предоставляет).

### Функциональность UI
- Таблица: время, коллекция, операция, длительность мс, план (`COLLSCAN`/`IXSCAN`), поля фильтра.
- Кнопка "Создать индекс" по полям фильтра с проверкой существующих.
- Для MongoDB: кнопка "Настроить порог" → форма с полем `slowms`.

---

## Раздел 5: Логи обновлений + откат

### Текущая проблема
`writeArchive` не сохраняет `archive_operation_id` — нет способа сгруппировать документы одного UPDATE-вызова для группового отката.

### Что изменить в `writeArchive`
Добавить поле `archive_operation_id` (UUID, генерируется один раз на вызов `archiveForUpdate`) ко всем документам операции:
```go
// Перед циклом
operationID := uuid.New().String()
archiveTime := time.Now().UnixNano()

for _, doc := range docs {
    copyDoc[...] = ...
    copyDoc["archive_operation_id"] = operationID
    copyDoc["archive_time"] = archiveTime
    copyDoc["source_collection"] = collection  // оригинальная коллекция
}
```

### Новый endpoint для отката UPDATE
`POST /api/v1/admin/restore/update` (только admin):
```go
type RestoreUpdateRequest struct {
    Collection         string `json:"collection"`
    ArchiveOperationID string `json:"archive_operation_id"`
}
```
**Логика:**
1. Прочитать все документы из `{collection}_update_archive` по `archive_operation_id`.
2. Для каждого документа: `UpdateDocuments` по `{"internal_id": doc["internal_id"]}` с данными из архива (исключить `archive_operation_id`, `archive_time`, `source_collection`).
3. Вернуть количество восстановленных документов.

### Функциональность UI
- Выбор коллекции → список операций обновления, сгруппированных по `archive_operation_id`.
- Колонки: время операции, коллекция, количество затронутых документов, `archive_operation_id`.
- Кнопка "Откатить" с `data-admin-confirm` → POST `/api/v1/admin/restore/update`.

---

## Раздел 6: Логи удалений + откат

### Аналогично Разделу 5
Добавить `archive_operation_id`, `archive_time`, `source_collection` в `{collection}_delete_archive`.

### Новый endpoint для отката DELETE
`POST /api/v1/admin/restore/delete`:
```go
type RestoreDeleteRequest struct {
    Collection         string `json:"collection"`
    ArchiveOperationID string `json:"archive_operation_id"`
}
```
**Логика:**
1. Прочитать все документы из `{collection}_delete_archive` по `archive_operation_id`.
2. Для каждого документа: `CreateDocuments` в оригинальную коллекцию (удалить `archive_operation_id`, `archive_time`, `source_collection`; сохранить оригинальный `internal_id`, `cr_time`).
3. Вернуть количество восстановленных документов.

**Отличие от UPDATE-отката:** тут `Create`, а не `Update`, т.к. документов больше нет.

---

## Раздел 7: Кастомные запросы

### Коллекция `_admin_custom_queries`
```go
type CustomQuery struct {
    InternalID  string                 `json:"internal_id"`
    Name        string                 `json:"name"`
    Collection  string                 `json:"collection"`
    Operation   string                 `json:"operation"`   // "find", "aggregate", "update", "delete"
    Body        map[string]interface{} `json:"body"`        // весь JSON запроса
    Description string                 `json:"description"`
    CrTime      int64                  `json:"cr_time"`
}
```

### Функциональность UI
- Таблица: имя, коллекция, операция, описание, дата создания.
- Форма добавления: имя, коллекция (выпадающий список из имеющихся), операция, textarea для JSON тела.
- Кнопка "Выполнить" → вызов соответствующего endpoint `/api/v1/documents/` с подстановкой тела.
- Кнопка "Удалить".

---

## Что нужно для реализации CloverDB-специфики

CloverDB (библиотека `github.com/ostafen/clover`) имеет следующие ограничения:

| Функция | MongoDB | CloverDB |
|---------|---------|----------|
| `collStats` | `RunCommand` | Только `Query(col).Count()` — только count, нет размера |
| `storageSize` | из `collStats` | **Недоступно** — CloverDB не предоставляет размер коллекции |
| `nindexes` | из `collStats` | Индексы v2 (`CreateIndex`/`DropIndex`) — можно хранить список в `_admin_index_registry` |
| `ListIndexes` | `collection.Indexes().List()` | **Нет API** — нужно вести `_admin_index_registry` вручную |
| Slow queries | `system.profile` | Только инструментирование — замер в каждом методе репозитория |
| Агрегация | MongoDB pipeline | **Нет** — CloverDB не поддерживает `$group`, `$match` и т.д. |

**Необходимые дополнения для CloverDB:**

1. **`_admin_index_registry`** — коллекция для хранения созданных индексов (т.к. CloverDB не возвращает список индексов через API). При `CreateIndex` — писать запись. При `DropIndex` — удалять.

2. **Статистика коллекций** — только поле `count` через `Query(col).Count()`. `storageSize` недоступен — отображать как "N/A".

3. **Медленные запросы** — оборачивать каждый метод CloverDB-репозитория в `time.Since`, если превышен порог — писать в `_admin_slow_queries`. Поля `keysExamined`/`docsExamined` — "N/A".

4. **Агрегация `_admin_query_stats`** — вместо MongoDB-агрегации читать через обычный `FindAll` + группировать в Go-коде.

---

## Новые конфиг-поля (`.env.example` / `config.template.yml`)

```yaml
storage:
  features:
    log_requests: false
    archive_changes: false
    track_query_stats: false        # счётчики по паттернам запросов
    slow_query_threshold_ms: 0      # 0 = отключено; для Mongo включает profiler
```

```
STORAGE_TRACK_QUERY_STATS=false
STORAGE_SLOW_QUERY_THRESHOLD_MS=0
```

---

## Расширение `StorageRepository` interface

```go
type StorageRepository interface {
    // существующие
    CreateDocuments(ctx context.Context, request CreateDocumentsRequest) ([]string, error)
    ReadDocuments(ctx context.Context, request ReadDocumentsRequest) ([]map[string]interface{}, int64, error)
    AggregateDocuments(ctx context.Context, request AggregateDocumentsRequest) ([]map[string]interface{}, int64, error)
    UpdateDocuments(ctx context.Context, request UpdateDocumentsRequest) (int64, error)
    DeleteDocuments(ctx context.Context, request DeleteDocumentsRequest) (int64, error)
    Close(ctx context.Context) error

    // новые для admin
    GetAdminCollectionStats(ctx context.Context) ([]CollectionStats, error)
    ListCollectionNames(ctx context.Context) ([]string, error)
    ListIndexes(ctx context.Context, collection string) ([]IndexInfo, error)
    CreateIndex(ctx context.Context, req CreateIndexRequest) error
    GetSlowQueries(ctx context.Context, limit int) ([]SlowQuery, error)
    SetSlowQueryThreshold(ctx context.Context, ms int) error
}
```

---

## Итоговая структура новых файлов

```
internal/
  admin.go                    # SetupAdmin(), все Page/Resource handlers
  admin_stats.go              # GetCollectionStats, ListIndexes
  admin_restore.go            # RestoreUpdate, RestoreDelete
  admin_queries.go            # CustomQuery CRUD, QueryStats
internal/mongo/
  admin.go                    # реализация admin-методов для MongoDB
internal/redis/
  admin.go                    # заглушки (Redis не поддерживает admin)
internal/clover/              # новый пакет — реализация CloverDB backend
  client.go
  repository.go
  admin.go                    # CloverDB-специфика для admin
types/
  admin.go                    # новые типы: CollectionStats, IndexInfo, SlowQuery, etc.
```
