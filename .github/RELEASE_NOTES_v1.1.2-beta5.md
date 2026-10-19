# S-UI-X Extended v1.1.2-beta5

This beta fixes Windows ARM64 release packaging and corrects Docker installation examples. Stable remains `v1.1.1`.

## Changes

- Fixed Windows ARM64 release packaging. Windows users should download the ZIP matching their architecture.
- Docker installation commands now use `ghcr.io/deposist/s-ui-x-extended:v1.1.2-beta5`.
- Includes the earlier 1.1.2 betas: Subscription outbounds with a selected primary server, optional failover, editable HWID and optional Device OS, Device model and OS version. Subscription behavior is unchanged from beta4.

Device details are saved with the outbound and reused for subscription downloads and automatic refreshes after a restart. Empty fields omit their headers. After changing device details or HWID, load the server list again, select the primary and optional backup, then save. Redirects to a different host, port or HTTP/HTTPS scheme do not receive device headers or HWID.

Xray JSON and proprietary encrypted subscriptions are not supported.

## Upgrade

Back up the panel database before installing this beta.

```sh
curl -fLsS \
  https://raw.githubusercontent.com/deposist/s-ui-x-extended/v1.1.2-beta5/install.sh \
  | sudo bash -s -- v1.1.2-beta5
```

Windows: extract the ZIP for your architecture and run `install-windows.bat` as Administrator.

Docker image: `ghcr.io/deposist/s-ui-x-extended:v1.1.2-beta5`.

Existing installations keep their settings and saved outbounds. Beta3 does not send device detail headers; if your provider requires them, subscription downloads can fail after rolling back. Versions before beta3 also lack HWID support. Before rolling back to a version older than beta2, remove Subscription outbounds and update their references, or restore the database backup from before the upgrade: those versions do not recognize this outbound type.

[Subscription setup, upgrade and rollback guide](https://github.com/deposist/s-ui-x-extended/blob/v1.1.2-beta5/docs/releases/v1.1.2-beta5.md)

## По-русски

Исправлена упаковка релиза Windows ARM64. Для Windows скачайте ZIP своей архитектуры и запустите `install-windows.bat` от имени администратора. Docker-команды теперь используют образ `ghcr.io/deposist/s-ui-x-extended:v1.1.2-beta5`. Стабильная версия остаётся `v1.1.1`.

Включены предыдущие beta-релизы 1.1.2: outbound «Подписка» с выбором основного сервера, failover, редактируемым HWID и необязательными сведениями об ОС, модели устройства и версии ОС. Работа подписок по сравнению с beta4 не изменилась.

Сведения сохраняются с outbound и используются при загрузке и автообновлении подписки после перезапуска. Пустые поля не отправляются. После изменения сведений или HWID заново загрузите список, выберите основной и при необходимости резервный сервер, затем сохраните outbound. При редиректе на другой хост, порт или протокол HTTP/HTTPS сведения об устройстве и HWID не передаются. Xray JSON и проприетарные зашифрованные подписки не поддерживаются.

Команда обновления приведена выше. Перед установкой сделайте резервную копию базы. Существующие настройки и outbound сохраняются. Beta3 не передаёт сведения об устройстве; если провайдер требует эти заголовки, после отката загрузка подписки может перестать работать. Версии до beta3 также не поддерживают HWID. Перед откатом на версии до beta2 удалите outbound «Подписка» и измените ссылающиеся на них правила и группы либо восстановите базу из копии до обновления.
