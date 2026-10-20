# S-UI-X Extended v1.1.2-beta7

This beta moves the core to sing-box 1.14.3. Stable remains `v1.1.1`.

## Changes

- Updated sing-box-extended to `v1.14.3-extended-2.7.8`, up from `v1.14.1-extended-2.7.6` in beta6.
- Rebuilt core startup around the lifecycle the new sing-box expects. NTP, the reference manager and the HTTP client now stop together with the core instead of after it.
- Added TrustTunnel and MASQUE fields from the core schema: `disable_path_mtu_discovery` for TrustTunnel, `handshake_timeout` and `use_l4_proxy` for MASQUE.
- The outbound URL validator now accepts punycode domains such as `http://xn--e1afmkfd.xn--p1ai/`. A TLD passes if it contains at least one letter.
- The log redactor masks credentials in lines without a URL scheme, so `dial user:pass@host:443` no longer reaches the log intact.

## Upgrade

Back up the panel database before installing this beta.

```sh
curl -fLsS \
  https://raw.githubusercontent.com/deposist/s-ui-x-extended/v1.1.2-beta7/install.sh \
  | sudo bash -s -- v1.1.2-beta7
```

Windows: extract the ZIP for your architecture and run `install-windows.bat` as Administrator.

Docker image: `ghcr.io/deposist/s-ui-x-extended:v1.1.2-beta7`.

Existing settings, clients and outbounds are retained. The configuration format is unchanged, so rolling back to beta6 needs only the database copy you made before upgrading.

## По-русски

Ядро обновлено до sing-box-extended `v1.14.3-extended-2.7.8`. В beta6 была версия `v1.14.1-extended-2.7.6`. Стабильная версия остаётся `v1.1.1`.

Загрузку ядра пришлось перестроить: компоненты запускаются в том порядке, который ожидает новая версия sing-box. NTP-сервис, менеджер ссылок и HTTP-клиент останавливаются вместе с ядром, а не после него.

В формах TrustTunnel и MASQUE появились новые поля из схемы ядра: `disable_path_mtu_discovery` в TrustTunnel, `handshake_timeout` и `use_l4_proxy` в MASQUE.

Валидатор внешних URL больше не отвергает домены в punycode. Адреса вида `http://xn--e1afmkfd.xn--p1ai/` проходят проверку: TLD принимается, если в нём есть хотя бы одна буква. Маскировщик логов вырезает учётные данные и из строк без схемы, так что `dial user:pass@host:443` больше не попадает в журнал открытым текстом.

Перед установкой сохраните резервную копию базы. Команда обновления приведена выше. Для Windows скачайте ZIP своей архитектуры и запустите `install-windows.bat` от имени администратора. Docker-образ: `ghcr.io/deposist/s-ui-x-extended:v1.1.2-beta7`.

Настройки, клиенты и outbound сохраняются. Формат конфигурации не менялся, поэтому для отката на beta6 достаточно копии базы, сделанной до обновления.
