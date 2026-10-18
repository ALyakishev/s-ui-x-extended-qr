# S-UI-X Extended v1.1.2-beta2

This beta adds subscription outbounds with a pinned primary server and optional failover. Stable remains `v1.1.1`.

## Changes

- In Outbounds, choose Subscription, enter the URL and click Load servers. Select a primary and, optionally, a different backup from the same subscription. Use the resulting outbound tag in routing rules or selectors.
- Supports HTTP(S) subscriptions used by Happ and Hiddify: share links, base64 lists, sing-box JSON and Clash/Mihomo YAML. Proprietary encrypted links are not supported.
- Health checks switch to the backup when the primary fails and return after the configured consecutive successful checks. If both servers fail, the current selection is retained without falling back to direct.
- Automatic refresh matches the selected servers by exact unique name. Failed downloads, invalid updates and missing selected servers retain the last saved configuration. Saving or applying changed server settings restarts the core and briefly interrupts connections.

The default refresh interval is `24h` (minimum `1m`). With failover, checks run every `30s` by default (minimum `5s`), and returning to the primary requires `2` successful checks. You can change these settings in the form.

Subscription nodes must be self-contained; references to local files and outbound detours are rejected. Routing rules and DNS from subscription profiles are not imported.

## Upgrade

Back up the panel database before installing this beta.

```sh
curl -fLsS \
  https://raw.githubusercontent.com/deposist/s-ui-x-extended/v1.1.2-beta2/install.sh \
  | sudo bash -s -- v1.1.2-beta2
```

The installer preserves the database and configuration. The service restart briefly interrupts active connections.

```sh
/usr/local/s-ui/sui -v
systemctl is-active s-ui
```

Windows: extract the ZIP for your architecture and run `install-windows.bat` as Administrator. Docker image: `ghcr.io/deposist/s-ui-x:v1.1.2-beta2`.

Before rolling back to an older version, remove Subscription outbounds and update their references, or restore the database backup from before the upgrade. Older versions do not recognize this outbound type.

[Setup and upgrade guide](https://github.com/deposist/s-ui-x-extended/blob/v1.1.2-beta2/docs/releases/v1.1.2-beta2.md)

## По-русски

Добавлен outbound типа «Подписка» с выбором конкретного основного сервера и резервного из той же подписки. Введите URL, нажмите «Загрузить серверы», выберите узлы и сохраните. В правилах маршрутизации и селекторах используйте тег этого outbound.

Поддерживаются HTTP(S)-подписки Happ и Hiddify: ссылки, списки base64, sing-box JSON и Clash/Mihomo YAML. Проприетарные зашифрованные ссылки не поддерживаются. Настройки серверов не должны ссылаться на локальные файлы или outbound detour; правила и DNS из подписки не импортируются.

При отказе основного трафик переключается на резервный. Возврат происходит после заданного числа успешных проверок подряд. Если недоступны оба, остаётся текущий выбор без переключения на direct. По умолчанию проверки идут раз в `30s` (минимум `5s`), возврат требует `2` успешных проверок.

Подписка обновляется раз в `24h` (минимум `1m`). Серверы привязаны к точным уникальным именам: ошибка загрузки, некорректное обновление или исчезновение выбранного узла сохраняют последнюю конфигурацию. Сохранение и применение изменившихся настроек перезапускают ядро и кратковременно прерывают соединения.

Команда обновления приведена выше. Перед установкой сделайте резервную копию базы. Для отката удалите новый тип outbound и измените ссылающиеся на него правила и группы либо восстановите базу из копии до обновления.
