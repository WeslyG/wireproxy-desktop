# Публикация в WinGet и Homebrew

Новые релизы используют тег `X.Y.Z`. Сборка публикует версионные установщики Windows, ZIP с приложением macOS и файл `SHA256SUMS-X.Y.Z` в GitHub Releases. После релиза workflow **Deliver packages to WinGet and Homebrew** скачивает эти файлы, сверяет SHA-256 и готовит манифесты. У каждого канала свой переключатель в **Settings → Secrets and variables → Actions**.

Ранее выпущенные сборки распознают только теги с `v`. При прямом переходе на новый формат их нужно один раз обновить вручную. Обновлятор в новых сборках понимает оба формата.

## WinGet

1. Укажите лицензию Wireproxy Desktop в переменной `WINGET_PACKAGE_LICENSE`. Файл `LICENSE.wireproxy-awg` относится к зависимости wireproxy-awg.
2. Создайте классический GitHub PAT со scope `public_repo` и сохраните его в секрете `WINGET_CREATE_GITHUB_TOKEN`. [Настройка токена WingetCreate](https://github.com/microsoft/winget-create/blob/main/doc/token.md)
3. Установите переменную `PUBLISH_WINGET=true`. После публикации релиза workflow отправит PR с манифестами `WeslyG.WireproxyDesktop` в `microsoft/winget-pkgs`. Ссылка на PR появится в журнале шага **Submit WinGet manifest** и в сводке запуска Actions.
4. Дождитесь меток `Validation-Completed` и `Moderator-Approved`, затем слияния PR и публикации в источнике `winget`. Если при первой подаче Microsoft CLA bot попросит принять соглашение, это делает автор PR со своего аккаунта. [Проверки и модерация](https://github.com/microsoft/winget-pkgs/blob/master/doc/Validation.md) · [Первая подача и CLA](https://github.com/microsoft/winget-pkgs/blob/master/doc/FirstContribution.md)

Установщик работает в профиле текущего пользователя, создаёт ярлык в меню «Пуск» и регистрирует удаление приложения. Обновлять установленные версии можно через WinGet или запуском более нового установщика. Профили остаются в хранилище текущего пользователя.

```powershell
winget install --id WeslyG.WireproxyDesktop --exact --source winget
winget upgrade --id WeslyG.WireproxyDesktop --exact --source winget
winget uninstall --id WeslyG.WireproxyDesktop --exact --source winget
```

## Homebrew

1. Создайте публичный репозиторий `WeslyG/homebrew-tap` и инициализируйте ветку `main`. Добавьте fine-grained PAT с правом **Contents: Read and write** для этого репозитория в секрет `HOMEBREW_TAP_TOKEN`. Workflow обновляет файл `Casks/wireproxy-desktop.rb` в `main`. [Устройство Homebrew tap](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap)
2. Установите `PUBLISH_HOMEBREW=true`. Перед обновлением tap workflow проверит SHA-256, архитектуру, версию и подпись ad hoc каждого архива, а также установку и удаление cask на Apple Silicon.

Homebrew сохраняет карантин для cask. При первом запуске macOS может запросить подтверждение для сборки с подписью ad hoc. Инструкция по первому запуску есть в [README](../README.md#первый-запуск-на-macos). [Как Homebrew проверяет cask](https://docs.brew.sh/Supply-Chain-Security) · [Открытие приложения в macOS](https://support.apple.com/ru-ru/102445)

```sh
brew install --cask WeslyG/tap/wireproxy-desktop
brew upgrade --cask WeslyG/tap/wireproxy-desktop
brew uninstall --cask WeslyG/tap/wireproxy-desktop
```

## Проверка перед включением публикации

- Для локальной сборки установщика понадобится Inno Setup 6 (от версии 6.3) и собранный Windows-бинарник соответствующей архитектуры. Например, `./scripts/build-windows-installer.ps1 -Tag 1.2.3 -Architecture amd64 -Binary ./dist/wireproxy-desktop.exe -OutputDirectory ./dist`. Для ARM64 используйте `-Architecture arm64` и бинарник ARM64. При коммерческом использовании условия лицензии Inno Setup описаны на сайте разработчика. [Inno Setup 6](https://jrsoftware.org/inno-setup/downloads#stable)
- На Python 3.11+ запустите `python scripts/generate-package-manifests.py 1.2.3 ./release-assets ./dist/packages --license "Лицензия проекта"`. В `release-assets` должны лежать два версионных установщика Windows и два версионных ZIP для macOS. В Windows проверьте каталог `dist/packages/manifests/w/WeslyG/WireproxyDesktop/1.2.3` командой `winget validate --manifest <каталог>`. [Команда WinGet validate](https://learn.microsoft.com/windows/package-manager/winget/validate)
- В Actions можно запустить **Deliver packages to WinGet and Homebrew** вручную для вышедшего стабильного тега. С выключенным `publish` workflow сохранит только артефакт `package-manifests-X.Y.Z`. С включённым `publish` он повторит доставку актуального релиза в каналы, разрешённые переменными репозитория.
- Перед объявлением каналов проверьте обновление через менеджер пакетов с предыдущей версии, запуск приложения и сохранность профилей после удаления и повторной установки.
