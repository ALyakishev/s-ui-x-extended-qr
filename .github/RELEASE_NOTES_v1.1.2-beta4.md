# S-UI-X Extended v1.1.2-beta4

This beta adds device details to Subscription outbounds. Stable remains `v1.1.1`.

## Changes

You can now enter Device OS, Device model and OS version alongside HWID. For example:

```http
X-Device-OS: Android
X-Device-Model: Samsung Galaxy S24
X-Ver-OS: 14
```

All three fields are optional and editable. The panel saves them with the outbound and reuses them for server-list downloads and automatic refreshes after a restart. Empty fields do not send a header; the panel does not infer values from your browser or the server.

After changing device details, click Load servers, select the primary and optional backup again, then save. Redirects to a different host, port or HTTP/HTTPS scheme do not receive the device headers or HWID.

HWID, failover and supported subscription formats are unchanged. Xray JSON and proprietary encrypted subscriptions are not supported.

## Upgrade

Back up the panel database before installing this beta.

```sh
curl -fLsS \
  https://raw.githubusercontent.com/deposist/s-ui-x-extended/v1.1.2-beta4/install.sh \
  | sudo bash -s -- v1.1.2-beta4
```

The installer preserves the database and configuration. Restarting the service briefly interrupts active connections.

```sh
/usr/local/s-ui/sui -v
systemctl is-active s-ui
```

Windows: extract the ZIP for your architecture and run `install-windows.bat` as Administrator. Docker image: `ghcr.io/deposist/s-ui-x-extended:v1.1.2-beta4`.

Beta3 does not send device detail headers. If your provider requires them, subscription downloads can fail after rolling back. Versions before beta3 also lack HWID support. Before rolling back to a version older than beta2, remove Subscription outbounds and update their references, or restore the database backup from before the upgrade: those versions do not recognize this outbound type.

[Device details setup and upgrade guide](https://github.com/deposist/s-ui-x-extended/blob/v1.1.2-beta4/docs/releases/v1.1.2-beta4.md)

## По-русски

В outbound «Подписка» рядом с HWID добавлены необязательные поля «ОС устройства», «Модель устройства» и «Версия ОС». Например, значения `Android`, `Samsung Galaxy S24` и `14` передаются в заголовках `X-Device-OS`, `X-Device-Model` и `X-Ver-OS`.

Поля можно редактировать. Панель сохраняет значения и использует их при загрузке списка серверов и автообновлении подписки после перезапуска. Пустые поля не отправляются. ОС и модель не определяются автоматически по браузеру или серверу.

После изменения нажмите «Загрузить серверы», заново выберите основной и при необходимости резервный сервер, затем сохраните outbound. При редиректе на другой хост, порт или протокол HTTP/HTTPS сведения об устройстве и HWID не передаются. Failover и форматы подписок не изменились; Xray JSON и проприетарные зашифрованные подписки не поддерживаются.

Команда обновления приведена выше. Перед установкой сделайте резервную копию базы. Beta3 не передаёт сведения об устройстве; если провайдер требует эти заголовки, после отката загрузка подписки может перестать работать. Версии до beta3 также не поддерживают HWID, а версии до beta2 не распознают outbound «Подписка». Перед откатом на версии до beta2 удалите эти outbound и измените ссылающиеся на них правила и группы либо восстановите базу из копии до обновления.
