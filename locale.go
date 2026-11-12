package main

import (
	"strings"
	"sync"
)

var systemLanguage = sync.OnceValue(func() string {
	for _, locale := range preferredLocales() {
		language := strings.FieldsFunc(strings.ToLower(locale), func(r rune) bool {
			return strings.ContainsRune("-_.@", r)
		})
		if len(language) != 0 {
			switch language[0] {
			case "en", "ru":
				return language[0]
			}
		}
	}
	return "en"
})

var russian = map[string]string{
	"Keychain access": "Доступ к Связке ключей",
	"System language": "Язык системы",
	"Continue":        "Продолжить",
	"After you continue, macOS may ask for Keychain access. Wireproxy Desktop uses its own Keychain entry to load and save your profiles and proxy settings.": "После нажатия «Продолжить» macOS может запросить доступ к Связке ключей. Wireproxy Desktop использует собственную запись в Связке ключей для загрузки и сохранения ваших профилей и настроек прокси.",
	"Protected storage":                                                            "Защищённое хранилище",
	"Opening protected storage…":                                                   "Открываю защищённое хранилище…",
	"Saving to protected storage…":                                                 "Сохраняю в защищённое хранилище…",
	"Could not open protected storage":                                             "Не удалось открыть защищённое хранилище",
	"Could not save to protected storage":                                          "Не удалось сохранить в защищённое хранилище",
	"Unlock protected storage and restart before saving changes.":                  "Разблокируйте защищённое хранилище и перезапустите приложение, чтобы сохранить изменения.",
	"An older configuration file still exists and differs from protected storage.": "Старый файл настроек отличается от данных в защищённом хранилище.",
	"Could not inspect the old configuration file.":                                "Не удалось прочитать старый файл настроек.",
	"The old configuration file changed during migration.":                         "Старый файл настроек изменился во время переноса.",
	"Could not remove the old configuration file.":                                 "Не удалось удалить старый файл настроек.",
	"Tunnel unavailable":                                                           "Туннель недоступен",
	"Authentication":                                                               "Авторизация",
	"Copy SOCKS5 URL":                                                              "Копировать URL SOCKS5",
	"SOCKS5 URL copied. Paste it into the app’s proxy settings.":                   "URL SOCKS5 скопирован. Вставьте его в настройки прокси нужного приложения.",
	"HTTPS works through the tunnel.":                                              "HTTPS через туннель работает.",
	"AmneziaWG engine":                                                             "Движок AmneziaWG",
	"Latest published release":                                                     "Последний опубликованный релиз",
	"Automatic installation is available in release builds.":                       "Автоустановка доступна в релизных сборках.",
	"Wireproxy is using this port":                                                 "Wireproxy уже использует этот порт",
	"Wireproxy is changing the listener state":                                     "Wireproxy запускает или останавливает прослушивание",
	"Checking HTTPS through SOCKS5…":                                               "Проверяю HTTPS через SOCKS5…",
	"SOCKS5 test passed":                                                           "Тест SOCKS5 пройден",
	"SOCKS5 test failed":                                                           "Тест SOCKS5 не пройден",
	"Follow log":                                                                   "Следить за журналом",
	"Listener":                                                                     "Прослушивание",
	"Interface language":                                                           "Язык интерфейса",
	"Profile protocol":                                                             "Протокол профиля",
	"Connection test timed out.":                                                   "Истекло время проверки подключения.",
	"Could not run the connection test.":                                           "Не удалось запустить проверку подключения.",
	"Could not read the connection test result.":                                   "Не удалось прочитать результат проверки.",
	"← Back":                    "← Назад",
	"Check connection":          "Проверить подключение",
	"Copy configuration":        "Копировать конфиг",
	"Configuration copied.":     "Конфигурация скопирована.",
	"Proxy":                     "Прокси",
	"Language":                  "Язык",
	"Updates":                   "Обновления",
	"Language saved.":           "Язык сохранён.",
	"Use SOCKS5 authentication": "Использовать авторизацию SOCKS5",
	"All IPv4 interfaces":       "Все интерфейсы IPv4",
	"All IPv6 interfaces":       "Все интерфейсы IPv6",
	"Could not list interfaces": "Не удалось получить список интерфейсов",
	"Check port":                "Проверить доступность",
	"Port is available":         "Порт доступен",
	"Cannot bind to this port":  "Не удалось открыть порт",
	"Use port":                  "Выбрать порт",
	"Suggested port selected.":  "Предложенный порт выбран.",
	"What's new":                "Что нового",
	"Open release on GitHub":    "Открыть релиз на GitHub",
	"Proxy off":                 "Прокси отключён",
	"Proxy on":                  "Прокси включён",
	"Open Wireproxy Desktop":    "Открыть Wireproxy Desktop",
	"Quit":                      "Выйти",
	"Servers":                   "Серверы",
	"Settings":                  "Настройки",
	"Logs":                      "Журнал",
	"Saved servers":             "Мои серверы",
	"+ Add":                     "+ Добавить",
	"No servers yet":            "Список пуст",
	"Add a server":              "Добавьте сервер",
	"Add server":                "Новый сервер",
	"Edit server":               "Редактирование сервера",
	"Server name":               "Название сервера",
	"e.g. Home server":          "Например, домашний сервер",
	"Paste an Amnezia vpn:// link or the contents of a WireGuard configuration file.": "Вставьте ссылку Amnezia vpn:// или содержимое файла WireGuard.",
	"Show configuration":              "Показать конфигурацию",
	"Convert to INI":                  "В INI",
	"Converted to INI.":               "Конфигурация переведена в INI.",
	"Conversion failed":               "Не удалось преобразовать ссылку",
	"Paste a complete vpn:// link.":   "Вставьте полную ссылку vpn://.",
	"Save server":                     "Сохранить",
	"Check profile":                   "Проверить",
	"Checking…":                       "Проверка…",
	"Checking the tunnel…":            "Проверяю туннель…",
	"Tunnel test failed":              "Проверка туннеля не прошла",
	"Enter a configuration to check.": "Вставьте конфигурацию для проверки.",
	"Invalid profile or unresolved endpoint. Check the configuration.": "Профиль некорректен или адрес сервера не разрешается. Проверьте конфигурацию.",
	"Cancel":               "Отмена",
	"Connect":              "Подключить",
	"Disconnect":           "Отключить",
	"Disconnecting…":       "Отключение…",
	"Disconnected":         "Отключено",
	"Starting":             "Подключение",
	"Stopping":             "Остановка",
	"SOCKS5 listening":     "SOCKS5 запущен",
	"Tunnel established":   "Туннель установлен",
	"Proxy failed":         "Ошибка прокси",
	"See Logs for details": "Подробности в журнале",
	"Test SOCKS5":          "Тест SOCKS5",
	"Testing…":             "Проверка…",
	"Default server":       "Основной сервер",
	"Set as default":       "Сделать основным",
	"Clear default":        "Снять отметку",
	"Protocol":             "Протокол",
	"Unknown":              "Не определён",
	"Disconnect the current proxy before starting another server.": "Отключите текущий прокси, чтобы запустить другой сервер.",
	"Edit":            "Изменить",
	"Remove":          "Удалить",
	"Confirm removal": "Подтвердить удаление",
	"Server saved.":   "Сервер сохранён.",
	"Server removed.": "Сервер удалён.",
	"A server with this name already exists.":            "Сервер с таким названием уже есть.",
	"Enter a server name and a WireGuard configuration.": "Укажите название сервера и конфигурацию WireGuard.",
	"Proxy settings":        "Настройки прокси",
	"SOCKS5 listen address": "Адрес прослушивания SOCKS5",
	"SOCKS5 port":           "Порт SOCKS5",
	"Use 0.0.0.0 for all IPv4 interfaces or :: for IPv6.": "0.0.0.0 — все интерфейсы IPv4, :: — IPv6.",
	"SOCKS5 username":        "Логин SOCKS5",
	"SOCKS5 password":        "Пароль SOCKS5",
	"Optional":               "Необязательно",
	"Verbose WireGuard logs": "Подробный журнал WireGuard",
	"Save settings":          "Сохранить настройки",
	"Settings saved.":        "Настройки сохранены.",
	"Settings saved. Reconnect to apply them.":             "Настройки сохранены. Переподключитесь, чтобы их применить.",
	"Enter a port between 1 and 65535.":                    "Укажите порт от 1 до 65535.",
	"Enter an IPv4 or IPv6 listen address.":                "Укажите адрес IPv4 или IPv6.",
	"Enter both SOCKS5 credentials, up to 255 bytes each.": "Заполните оба поля, логин и пароль не длиннее 255 байт каждое.",
	"Profiles file": "Файл профилей",
	"Fix the profiles file and restart before saving changes.": "Исправьте файл профилей и перезапустите приложение.",
	"Activity log": "Журнал событий",
	"Copy logs":    "Копировать журнал",
	"Clear":        "Очистить",
	"Logs copied.": "Журнал скопирован.",
	"Connect to a server to see activity here.": "Подключитесь к серверу, чтобы увидеть события.",
	"App updates":         "Обновления приложения",
	"Check for updates":   "Проверить обновления",
	"Install and restart": "Установить и перезапустить",
	"Download the new app from GitHub and replace your current copy.":      "Скачайте новую версию с GitHub и замените приложение.",
	"Updates are available in tagged release builds.":                      "Обновление доступно в релизных сборках.",
	"Checking GitHub for updates…":                                         "Проверяю обновления на GitHub…",
	"Downloading and verifying the update…":                                "Скачиваю и проверяю обновление…",
	"Restarting to install the update…":                                    "Перезапуск для установки обновления…",
	"No newer release found.":                                              "Новых версий нет.",
	"Installing restarts Wireproxy and disconnects the proxy.":             "При установке Wireproxy перезапустится, а прокси отключится.",
	"Available version":                                                    "Доступна версия",
	"Update installation failed. The previous version is still available.": "Не удалось установить обновление. Доступна предыдущая версия.",
}

func (d *desktop) tr(value string) string {
	language := d.cfg.Settings.Language
	if language == "" || language == "system" {
		language = systemLanguage()
	}
	if language == "ru" {
		if translated, ok := russian[value]; ok {
			return translated
		}
	}
	return value
}
