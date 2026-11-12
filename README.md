# Wireproxy Desktop

[English](README.en.md)

- Настольное приложение, которое поднимает SOCKS5-прокси через ваш сервер WireGuard или AmneziaWG.

![Wireproxy Desktop со списком демонстрационных серверов, основным профилем и кнопкой подключения](assets/screenshot.jpg)

## Скачать

[Последний релиз](https://github.com/WeslyG/wireproxy-desktop/releases/latest) · [Все релизы](https://github.com/WeslyG/wireproxy-desktop/releases) · [SHA256SUMS](https://github.com/WeslyG/wireproxy-desktop/releases/latest/download/SHA256SUMS)

| Система | x64 | ARM64 |
| --- | --- | --- |
| Windows | [Скачать x64](https://github.com/WeslyG/wireproxy-desktop/releases/latest/download/wireproxy-desktop-windows-amd64.exe) | [Скачать ARM64](https://github.com/WeslyG/wireproxy-desktop/releases/latest/download/wireproxy-desktop-windows-arm64.exe) |
| macOS | [Скачать x64](https://github.com/WeslyG/wireproxy-desktop/releases/latest/download/wireproxy-desktop-macos-amd64) | [Скачать ARM64](https://github.com/WeslyG/wireproxy-desktop/releases/latest/download/wireproxy-desktop-macos-arm64) |
| Linux | [Скачать x64](https://github.com/WeslyG/wireproxy-desktop/releases/latest/download/wireproxy-desktop-linux-amd64) | [Скачать ARM64](https://github.com/WeslyG/wireproxy-desktop/releases/latest/download/wireproxy-desktop-linux-arm64) |

## Системные требования

Готовая сборка запускается от обычного пользователя. Установщик, Go и драйвер WireGuard для запуска не нужны.

| ОС | Целевая среда | Что нужно для запуска |
| --- | --- | --- |
| Windows | Windows 11, x64 или ARM64 | Запустите скачанный `.exe`. Дополнительных компонентов нет. |
| macOS | macOS 15, Intel или Apple Silicon | Бинарник запускается из Терминала, если политика Gatekeeper разрешает запуск. Для хранения профилей используется встроенная связка ключей. |
| Linux | Ubuntu 24.04 Desktop, x64 или ARM64 | Нужна графическая сессия X11 или Wayland с системными библиотеками EGL/OpenGL ES и доступное хранилище Secret Service. Если в вашей среде его нет, установите GNOME Keyring или включите совместимый сервис KWallet. Для трея нужен StatusNotifier-хост. |

Таблица указывает [целевые среды сборки](.github/workflows/build.yml). На других дистрибутивах Linux нужны совместимые системные библиотеки. На macOS и Linux скачанному файлу может потребоваться право на выполнение. Например, для Linux x64 выполните `chmod +x ./wireproxy-desktop-linux-amd64`, затем `./wireproxy-desktop-linux-amd64`.

Сборки для macOS имеют локальную подпись ad hoc. Подпись Developer ID и нотариализация Apple отсутствуют, поэтому Gatekeeper может заблокировать скачанный файл при первом запуске. [Требования Apple к распространению](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)

## Подключение

1. В разделе **Серверы** добавьте профиль из ссылки `vpn://` или файла `.conf`. Вкладка **INI** преобразует ссылку в конфигурацию, которую можно скопировать.
2. Нажмите **Подключить**. Отметьте **Основной сервер**, чтобы быстро подключаться к нему с других вкладок.
3. Укажите `socks5://127.0.0.1:1080` в браузере или другом приложении либо скопируйте фактический URL из Wireproxy Desktop.

Приложение работает с **WireGuard v1** и профилями **AmneziaWG 1.0, 1.5, 2.0, 3.0 и 3.1**. В карточке сервера отдельно показаны версия профиля и версия встроенного движка. Для INI версия профиля определяется по параметрам.

В настройках можно выбрать язык, адрес и порт SOCKS5, проверить доступность порта и включить авторизацию по логину и паролю. Для прослушивания доступны локальные интерфейсы и все интерфейсы IPv4 или IPv6.

## Хранение секретов

| ОС | Где хранятся профили, ключи и пароль SOCKS5 |
| --- | --- |
| Windows | Шифротекст DPAPI в реестре текущего пользователя |
| macOS | Системная связка ключей (Keychain) |
| Linux | Secret Service, например GNOME Keyring или KWallet |

При первом запуске старая конфигурация переносится в защищённое хранилище. Приложение удаляет старый JSON после проверки сохранённых данных. `--portable` создаёт отдельную запись хранилища ОС для каталога запуска.

## Обновления

Релизная сборка при запуске проверяет GitHub Releases. В **Настройки → Обновления** доступны ручная проверка, список изменений и ссылка на релиз. По команде **Установить и перезапустить** приложение скачивает файл для вашей платформы, проверяет SHA-256 и перезапускается. Для замены файла нужна запись в каталог программы.

## Диагностика

**Проверить подключение** тестирует черновик профиля через HTTPS. **Тест SOCKS5** проверяет рабочий прокси после рукопожатия. В журнале видны состояние туннеля, события WireGuard и результаты проверок. Доступны подробный режим, копирование и автоматическая прокрутка.

## Конфиденциальность

**Без аналитики, телеметрии, трекеров и автоматической отправки журналов.** Журнал хранится в памяти. Проверка обновлений обращается к GitHub, а диагностика по кнопке делает HTTPS-запрос к `example.com` через проверяемый туннель.

## Стек и сборка

Go, [Gio](https://gioui.org/) для интерфейса, [wireproxy-awg](https://github.com/WeslyG/wireproxy-awg) и AmneziaWG-Go для туннеля. GitHub Actions собирает шесть платформенных вариантов. Требуется Go 1.26.11 или новее.

Для разработки установите Docker и расширение **Dev Containers** в VS Code, откройте репозиторий и выберите **Dev Containers: Reopen in Container**. Контейнер включает Go и зависимости Gio для Linux. В его терминале выполните `go test -tags novulkan ./...` и `go build -tags novulkan -o ./wireproxy-desktop .`. Для проверки окна и интеграции с хранилищем секретов запускайте сборку в графической сессии целевой ОС.

| Платформа | Команда |
| --- | --- |
| Windows | `go build -trimpath -ldflags="-s -w -H=windowsgui" -o wireproxy-desktop.exe .` |
| macOS | `CGO_ENABLED=1 go build -trimpath -o wireproxy-desktop .` |
| Linux | `CGO_ENABLED=1 go build -tags novulkan -trimpath -o wireproxy-desktop .` |

Для сборки на macOS нужны Xcode Command Line Tools. Для сборки на Linux нужны компилятор C, `pkg-config` и [пакеты разработки для Gio](https://gioui.org/doc/install/linux). При закрытии окна прокси останавливается, а при наличии трея приложение остаётся в фоне. Настройки прослушивания применяются при следующем подключении.
