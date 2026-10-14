# S-UI-X Extended v1.1.2-beta1

This prerelease reduces MTProxy log noise and updates Go security fixes. Stable remains `v1.1.1`.

## Changes

- Rejected MTProxy authentication (`incorrect client random`) now uses the debug log level instead of error.
- Stream start/finish messages and relay shutdown with `io: read/write on closed pipe` now use debug instead of info.
- Other handshake and transport failures retain their existing severity. Enable debug logging when diagnosing rejected connections.
- Built with Go `1.26.9` and `golang.org/x/net v0.60.0`, fixing known HTTP/HTTP2, TLS, multipart parsing and template vulnerabilities.

MTProxy authentication, routing, client secrets and the database schema are unchanged.

## Upgrade

```sh
curl -fLsS \
  https://raw.githubusercontent.com/deposist/s-ui-x-extended/v1.1.2-beta1/install.sh \
  | sudo bash -s -- v1.1.2-beta1
```

The installer preserves the database and configuration. Restarting the service briefly interrupts active connections.

```sh
/usr/local/s-ui/sui -v
systemctl is-active s-ui
```

## По-русски

В этой beta уменьшен шум журнала MTProxy и обновлены исправления безопасности Go. Стабильная версия остаётся `v1.1.1`.

Отказ проверки подлинности `incorrect client random`, начало и завершение потоков, а также закрытие relay с `io: read/write on closed pipe` переведены на уровень debug. Остальные ошибки рукопожатия и транспорта сохраняют прежний уровень. Для диагностики отклонённых соединений включите debug.

Сборки используют Go `1.26.9` и `golang.org/x/net v0.60.0` с исправлениями известных уязвимостей HTTP/HTTP2, TLS, multipart-парсинга и шаблонов. Проверка подлинности MTProxy, маршрутизация, секреты клиентов и схема базы данных не меняются. Команда обновления приведена выше; установщик сохраняет базу и настройки. Перезапуск службы кратковременно прерывает активные соединения.
