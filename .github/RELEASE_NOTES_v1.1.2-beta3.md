# S-UI-X Extended v1.1.2-beta3

This beta adds an optional HWID to Subscription outbounds. Stable remains `v1.1.1`.

## Changes

- Enter a provider-authorized HWID manually, or click Assign automatically to generate a UUID. You can edit the generated value before saving.
- The panel saves the identifier with the outbound and sends it as `X-HWID` when loading servers and refreshing the subscription. Restarting the panel does not change it. Leave the field empty to omit the header.
- Changing HWID clears the server selections in the form. Load the list again, select the primary and optional backup, then save. Your provider may count a different HWID as another device.
- Redirects to a different host, port or HTTP/HTTPS scheme do not receive the identifier.

Subscription formats are unchanged: share links, base64 lists, sing-box JSON and Clash/Mihomo YAML. Xray JSON and proprietary encrypted subscriptions are not supported.

## Upgrade

Back up the panel database before installing this beta.

```sh
curl -fLsS \
  https://raw.githubusercontent.com/deposist/s-ui-x-extended/v1.1.2-beta3/install.sh \
  | sudo bash -s -- v1.1.2-beta3
```

The installer preserves the database and configuration. Restarting the service briefly interrupts active connections.

```sh
/usr/local/s-ui/sui -v
systemctl is-active s-ui
```

Windows: extract the ZIP for your architecture and run `install-windows.bat` as Administrator. Docker image: `ghcr.io/deposist/s-ui-x:v1.1.2-beta3`.

Beta2 does not send HWID headers. If your provider requires one, subscription downloads will fail after rolling back to beta2. Versions before beta2 do not recognize Subscription outbounds: remove them and update their references before rolling back, or restore the database backup from before the upgrade.

[HWID setup and upgrade guide](https://github.com/deposist/s-ui-x-extended/blob/v1.1.2-beta3/docs/releases/v1.1.2-beta3.md)

## По-русски

В outbound «Подписка» появилось необязательное поле HWID. Введите разрешённый провайдером идентификатор или нажмите «Назначить автоматически», чтобы создать UUID. Сгенерированное значение можно отредактировать.

Панель сохраняет HWID вместе с outbound и передаёт его в заголовке `X-HWID` при загрузке серверов и автообновлении подписки. Перезапуск панели его не меняет. Если оставить поле пустым, заголовок не отправляется.

После смены HWID заново загрузите список и выберите основной и при необходимости резервный сервер, затем сохраните outbound. Провайдер может учитывать новый идентификатор как отдельное устройство. При редиректе на другой хост или порт, а также при смене HTTP/HTTPS идентификатор не передаётся.

Форматы подписок не изменились. Xray JSON и проприетарные зашифрованные подписки не поддерживаются.

Команда обновления приведена выше. Перед установкой сделайте резервную копию базы. Beta2 не передаёт HWID: если провайдер требует этот заголовок, загрузка подписки после отката перестанет работать. Версии до beta2 не распознают outbound «Подписка»; перед откатом удалите их и измените ссылающиеся на них правила и группы либо восстановите базу из копии до обновления.
