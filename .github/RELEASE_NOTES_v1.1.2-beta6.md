# S-UI-X Extended v1.1.2-beta6

This beta updates the core to sing-box 1.14.1. Stable remains `v1.1.1`.

## Changes

- Updated sing-box-extended to `v1.14.1-extended-2.7.6`, based on upstream `v1.14.1-extended-2.7.2`. AmneziaWG 3.1 options, the random-trailers handshake fix and live WireGuard peer management are preserved.
- Adapted Clash API mode handling and core log output to the new core interfaces.
- Includes Subscription outbounds with primary-server selection, optional failover, editable HWID and optional device details from the earlier 1.1.2 betas. The Windows ARM64 packaging fix is also included.

## Upgrade

Back up the panel database before installing this beta.

```sh
curl -fLsS \
  https://raw.githubusercontent.com/deposist/s-ui-x-extended/v1.1.2-beta6/install.sh \
  | sudo bash -s -- v1.1.2-beta6
```

Windows: extract the ZIP for your architecture and run `install-windows.bat` as Administrator.

Docker image: `ghcr.io/deposist/s-ui-x-extended:v1.1.2-beta6`.

Existing settings and saved outbounds are retained. Subscription behavior is unchanged from beta5. After changing HWID or device details, reload the server list, select the primary and optional backup, then save.

Before rolling back to a version older than beta2, remove Subscription outbounds and update their references, or restore the database backup from before the upgrade. Those versions do not recognize this outbound type.

[Subscription setup and rollback details](https://github.com/deposist/s-ui-x-extended/blob/v1.1.2-beta6/docs/releases/v1.1.2-beta5.md)

## По-русски

Ядро обновлено до sing-box 1.14.1. Сохранены параметры AmneziaWG 3.1, исправление handshake со случайными трейлерами и управление WireGuard-пирами без перезапуска ядра. Обработка режимов Clash API и вывод журналов адаптированы к новым интерфейсам. Стабильная версия остаётся `v1.1.1`.

Включены возможности outbound «Подписка» из предыдущих beta-релизов 1.1.2 и исправление упаковки Windows ARM64. Работа подписок по сравнению с beta5 не изменилась.

Перед установкой сохраните резервную копию базы. Команда обновления приведена выше. Для Windows скачайте ZIP своей архитектуры и запустите `install-windows.bat` от имени администратора. Docker-образ: `ghcr.io/deposist/s-ui-x-extended:v1.1.2-beta6`.

Настройки и сохранённые outbound сохраняются. После изменения HWID или сведений об устройстве заново загрузите список серверов, выберите основной и при необходимости резервный, затем сохраните outbound. Перед откатом на версии до beta2 удалите outbound «Подписка» и измените ссылающиеся на них правила и группы либо восстановите базу из копии до обновления.
