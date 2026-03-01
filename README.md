# LinkTracker

**LinkTracker** – Telegram-бот, который отслеживает изменения на веб-страницах и оперативно информирует пользователя о них.

## Инструкция для ассистента: запуск и настройка бота

1. **Клонировать репозиторий** и перейти в корень проекта.

2. **Настроить конфигурацию.** В корне проекта создать файл `.env` (он не коммитится). Обязательная переменная:
   ```properties
   APP_TELEGRAM_TOKEN=<токен_бота_от_@BotFather>
   ```
   Токен получают в [@BotFather](https://t.me/BotFather) в Telegram. Формат: `123456789:ABCdefGHI...`.

3. **Собрать проект:**
   ```bash
   make build
   ```
   Исполняемый файл бота появится в `./bin/bot`.

4. **Запустить бота:**
   ```bash
   ./bin/bot
   ```
   Либо без сборки: `go run ./cmd/bot`.

5. **Опционально:** для отладочных логов задать переменную окружения `DEBUG=1` перед запуском.

## Как добавить новую команду

Команды бота живут в `internal/bot/application/commands/` и реализуют интерфейс `Command` (пакет `commands`). Чтобы добавить команду:

1. **Создать файл** в `internal/bot/application/commands/`, например `mycommand.go`.

2. **Реализовать обработчик** с тремя методами:
   - `Name() string` — имя команды **без** слэша (пользователь будет вызывать `/mycommand`).
   - `Description() string` — краткое описание для меню команд в Telegram (показывается в списке при нажатии на «/»).
   - `Handle(action domain.Action, api *tgbotapi.BotAPI) (string, bool)` — логика команды: вернуть текст ответа и `true`, если его нужно отправить в чат.

   За образец можно взять [internal/bot/application/commands/start.go](./internal/bot/application/commands/start.go) или [help.go](./internal/bot/application/commands/help.go).

3. **Зарегистрировать команду** в [internal/bot/application/commands/registry.go](./internal/bot/application/commands/registry.go): добавить свой тип в слайс, возвращаемый функцией `All()`:
   ```go
   return []Command{
       Start{},
       Help{},
       MyCommand{},  // новая команда
   }
   ```

После этого команда автоматически появится в диспетчере и в меню бота (setMyCommands вызывается при старте).

Дополнительная информация по структуре проекта и конфигурации — в [HELP.md](./HELP.md).