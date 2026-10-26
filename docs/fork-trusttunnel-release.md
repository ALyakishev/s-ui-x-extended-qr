# Подготовка предварительного релиза форка и Debian 12

Этот документ — план публикации и инструкция для оператора. Подготовка не создаёт
тег, Release, Docker-образ и не изменяет VPS.

## Версия и исходный коммит

Проверена основная ветка `s-ui-x-extended` на коммите
`270eaa11e26c1c42e0c5a8e50296ba04b18ee6f8`. Она содержит объединённый PR #1.
Это SHA аудита, **не финальный SHA будущего релиза**: сначала требуется проверить
и объединить PR подготовки, затем зафиксировать точный merge SHA основной ветки.

Предложенный `v1.0.0-tt.1` свободен, но ниже текущего `1.1.2-beta7` по правилам
сравнения версий приложения; он также не совпадает с текущим `config/version`.
Используем `v1.1.2-tt.1`: он свободен в форке и upstream при проверке 26 октября
2026, сохраняет базовую версию upstream и выше `1.1.2-beta7`. Перед публикацией
проверить свободу повторно. `config/version` и корневые версии frontend package
и lockfile подготовлены как `1.1.2-tt.1`; зависимости не менялись. Исторические
upstream notes, README-примеры и Compose-образы не переименовываются в релиз форка.

## Цепочка существующей сборки

`release.yml` вызывает `windows.yml` и composite action `checkout-release-tag`.
Action валидирует SemVer, существующий тег, совпадение с `config/version`, SHA
события push и ранее разрешённым SHA для downstream jobs. Сам workflow тег не
создаёт. Дополнительная проверка требует принадлежности коммита истории нашей
основной ветки. Оператор выбирает **точный проверенный SHA**, не плавающую ветку.
Все release setup-go используют `go.mod` (`1.26.10`); Dockerfile также закреплён
на Go 1.26.10 по digest. Test Build читает тот же `go.mod`.

Сборка frontend: `npm ci`, lint, tests, build, verify:dist. Результат копируется
в `web/html` для `go:embed` в бинарник. Linux AMD64: CGO=1, musl/Cronet,
статическая внешняя линковка, полный набор extended tags, включая
`with_trusttunnel`, `with_naive_outbound`, QUIC, TLS, ACME и WireGuard. Это формат
для Debian 12 x86_64 без зависимости от версии glibc хоста; CA-сертификаты, tzdata
и необходимые протоколам права/сетевые утилиты на машине всё равно нужны.

Проверяется отсутствие ELF INTERP, runtime-версия AMD64, точный состав Linux
архива, его SHA256 и привязка manifest к версии/платформе/каналу/хешу. Windows
проверяет состав ZIP, PE архитектуру DLL/EXE и runtime AMD64. ARM/Windows ARM64
собираются, но не исполняются на родном железе этой цепочкой.

Source security gate: vet, tests, staticcheck, golangci-lint, gosec, govulncheck;
дополнительно AWG integration и govulncheck неурезанного AMD64 бинарника.
SBOM/SPDX и Trivy проверяют Linux **и Windows** архивы; provenance включает
архивы, SHA256, Linux manifests и SBOM. Аттестация — подписанный GitHub/Sigstore
bundle, а не отдельная GPG-подпись каждого файла.

Перед публикацией: ровно 27 файлов, локальные SHA256, повторная проверка тега,
сверка имён и authoritative SHA256 remote assets, отказ при попытке заменить
несовпадающий файл. Сначала draft, затем публикация только после проверок.
При сбое draft может остаться; исправленный код требует нового тега, опубликованные
артефакты не заменять.

Артефакты GitHub Release:

- `s-ui-linux-{amd64,arm64,armv7,armv6,armv5,386,s390x}.tar.gz` — 7 архивов;
  рядом `.sha256` и `.manifest.json` — ещё 14 файлов.
  Каждый архив: `s-ui/sui`, `s-ui/s-ui.service`, `s-ui/s-ui.sh`.
- `s-ui-windows-{amd64,arm64}.zip` и `.zip.sha256` — 4 файла. ZIP:
  `sui.exe`, `libcronet.dll`, `install-windows.bat`, `s-ui-windows.bat`,
  `uninstall-windows.bat`, `s-ui-windows.xml`, `README.md`.
- `s-ui-release.spdx.json`, `s-ui-release-attestation.json` — 2 файла.

Push тега также запускает самостоятельные `windows.yml`, `docker.yml` и аудиты.
Windows при push дополнительно соберётся отдельно от reusable-вызова — это
существующая избыточность. Docker публикует GHCR для linux/amd64 и linux/arm64;
для prerelease не обновляет `latest`. Release Smoke запускается отдельно вручную,
после появления Release; он не является предварительным блокирующим gate.

## Проверки и GitHub Settings перед публикацией

Для PR подготовки оставить обязательными существующие CI: Backend (включая
release regression tests), Frontend, Audit Go, Audit Frontend и semgrep-custom.
В Settings → Rules → Rulesets / Branch protection требовать успешные реальные
check contexts этих workflow и review для `s-ui-x-extended`, без bypass ботом.
Настройки required checks нельзя подтвердить/изменить без доступа к GitHub API.

Разрешить Actions и закреплённые SHA actions, разрешить job-scoped `contents:write`,
`id-token:write`, `attestations:write` для release workflow; для Docker ещё
`packages:write`. Для первой публикации проверить доступ к GHCR и attestations
для типа/видимости репозитория. Права выдаются workflow на job level, PAT не нужен.
Не отключать security gates ради выпуска. Убедиться, что нет политики, запрещающей
нужные actions или provenance. Ограничить создание/перезапись `v*` тегов ruleset.

Перед тегом выполнить общий [pre-tag gate](release-workflow-runbook.md).
Нужен полный release-compatible Test Build AMD64 на **будущем release SHA** с
последующей проверкой на отдельной Debian 12 VM/контейнере, не на действующем VPS:
`file`, `readelf -l`, `sui -v`, запуск панели с временной БД, capabilities,
выдача JSON и `tt://`/QR, импорт в Shadowrocket и реальное TLS-подключение.
Тест на устройстве особенно важен для SNI отличного от IP сервера и IPv6.

## Результаты локальной проверки подготовки

- Go 1.26.10: полная `go test -count=1 -timeout=10m ./...` и `go vet ./...` успешны.
- Race: `go test -race -count=1 ./util ./sub ./core/capabilities` успешен.
- Frontend: lint без предупреждений, 528 тестов, production build, проверка 123 assets успешны.
  Локально Node 24.19.0; workflow использует Node 25, его выполнение остаётся отдельной проверкой.
- Release regression suite и 10 новых packaging/workflow tests успешны; actionlint
  всех workflows, разбор YAML, Bash/Python syntax и `git diff --check` успешны.
- `govulncheck` по локальному снимку официальной базы от 20 октября 2026:
  0 достижимых уязвимостей, 3 недостижимые в импортируемых пакетах и 1 в модулях.
  Это не заменяет онлайн-аудит на момент публикации.
- Собран AMD64 бинарник с base release tags, включая TrustTunnel, CGO=1 и
  статической линковкой; реальный tar.gz прошёл checksum/manifest/inventory validator.
  Эта локальная сборка использует GCC/glibc, **без Naive и release musl/Cronet
  toolchain**, и не является публикуемым артефактом.
- В локальном Debian 12 AMD64 контейнере успешны version, embedded frontend,
  login, capabilities TrustTunnel и сохранённое `clientDelivery=json`.
  Проверялся образ `debian@sha256:7ff6ba31527b9e336162fa061746c366c4b2032f4d2cff2078bea43f78efe3f8`.
- Полная release-матрица, Trivy, генерация/проверка подписанного SBOM/provenance,
  privileged AWG integration и Shadowrocket device test локально не запускались.
  Release workflow не запускался. Публичной страницы выбранного Release нет (HTTP 404);
  отсутствие приватного draft следует проверить через GitHub перед тегом.

## Получение и проверка AMD64 архива (после публикации)

Все команды установки ниже выполняет оператор на выбранной машине. Они не
выполнялись в рамках подготовки. Сначала сохранить firewall/SSH доступ;
не открывать панель в интернет до настройки администратора и HTTPS.

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl tar gzip openssl tzdata binutils file
sudo -i
set -euo pipefail
umask 077
test "$(uname -m)" = x86_64
release_tag=v1.1.2-tt.1
release_repo=ALyakishev/s-ui-x-extended-qr
release_stage=$(mktemp -d /root/s-ui-release.XXXXXXXX)
cd "$release_stage"
for asset in s-ui-linux-amd64.tar.gz s-ui-linux-amd64.tar.gz.sha256 s-ui-linux-amd64.tar.gz.manifest.json s-ui-release-attestation.json; do
  curl --fail --location --proto '=https' --tlsv1.2 \
    --output "$asset" "https://github.com/$release_repo/releases/download/$release_tag/$asset"
done
sha256sum --check --strict s-ui-linux-amd64.tar.gz.sha256
```

На рабочей станции с GitHub CLI проверить подписанную provenance перед
исполнением бинарника (если CLI отсутствует на сервере, передать проверенный
архив по SSH и повторно сверить хеш):

```bash
gh attestation verify s-ui-linux-amd64.tar.gz \
  --repo ALyakishev/s-ui-x-extended-qr \
  --signer-workflow ALyakishev/s-ui-x-extended-qr/.github/workflows/release.yml \
  --bundle s-ui-release-attestation.json
```

Проверить в результате provenance `sourceRepositoryDigest`/source commit
на совпадение с SHA, зафиксированным при публикации, и идентификатор release run.
SHA256 сам по себе не подтверждает доверенного издателя.

Проверить список `tar -tzf`: только `s-ui/`, `s-ui/sui`, `s-ui/s-ui.service`,
`s-ui/s-ui.sh`, без ссылок/дополнительных файлов. Затем:

```bash
tar --extract --gzip --file s-ui-linux-amd64.tar.gz --no-same-owner --no-same-permissions
file s-ui/sui
readelf -l s-ui/sui > program-headers.txt
if grep -q INTERP program-headers.txt; then echo 'Not a static release binary' >&2; exit 1; fi
test "$(s-ui/sui -v | awk 'NR == 1 {print $NF}')" = "${release_tag#v}"
```

## Чистая установка Debian 12

Требуется действительно пустая установка. При наличии любой БД перейти к
процедуре обновления. Не выполнять upstream `install.sh`: даже с тегом форка он
запрашивает upstream asset. Меню `s-ui.sh` также содержит upstream installer.

```bash
test ! -e /usr/local/s-ui
test ! -e /etc/s-ui
test ! -e /etc/systemd/system/s-ui.service
test ! -e /etc/systemd/system/s-ui.service.d
install -d -m 0755 /usr/local/s-ui
install -d -m 0700 /usr/local/s-ui/db /etc/s-ui
install -m 0755 "$release_stage/s-ui/sui" /usr/local/s-ui/sui
install -m 0644 "$release_stage/s-ui/s-ui.service" /etc/systemd/system/s-ui.service
{
  printf 'SUI_SECRETBOX_KEY=%s\n' "$(openssl rand -base64 32)"
  printf 'AWG_KEY_ENC=%s\n' "$(openssl rand -base64 32)"
} > /etc/s-ui/secretbox.env
chmod 0600 /etc/s-ui/secretbox.env
install -d -m 0755 /etc/systemd/system/s-ui.service.d
printf '[Service]\nEnvironmentFile=/etc/s-ui/secretbox.env\n' \
  > /etc/systemd/system/s-ui.service.d/10-secretbox-env.conf
cd /usr/local/s-ui
set -a
. /etc/s-ui/secretbox.env
set +a
./sui migrate
./sui setting -port 2095 -path /app/
./sui admin -reset
systemctl daemon-reload
systemctl enable --now s-ui
systemctl is-active --quiet s-ui
./sui -v
```

Сохранить сгенерированный пароль локально, не в общие логи. Открыть панель через
SSH tunnel `ssh -L 2095:127.0.0.1:2095 root@SERVER`, настроить HTTPS/ограничение
доступа и только затем необходимые сетевые порты. Subscription по умолчанию
использует порт 2096; разрешать его только согласно своей конфигурации.
Сохранить `/etc/s-ui/secretbox.env` отдельно в защищённой резервной копии:
без прежних ключей зашифрованная конфигурация не восстанавливается.

## Обновление с предварительной резервной копией

Сначала скачать и проверить архив, как выше, пока старая служба работает.
Определить фактическую конфигурацию без вывода секретов в общие логи:
`systemctl cat s-ui`, `systemctl show s-ui -p FragmentPath -p DropInPaths -p WorkingDirectory`.
Далее пример **стандартной** установки с БД `/usr/local/s-ui/db` и ключами
`/etc/s-ui`. Если задан `SUI_DB_FOLDER`, внешний EnvironmentFile, сертификаты,
unit в другом каталоге или альтернативный бинарник — включить реальные пути в
backup и скорректировать команды до остановки. Не считать стандартный backup
полным для нестандартной установки. Зафиксировать исходные active/enabled states.

```bash
backup_dir="/root/s-ui-backup-$(date -u +%Y%m%dT%H%M%SZ)"
install -d -m 0700 "$backup_dir"
systemctl is-active s-ui > "$backup_dir/active-state.txt" || true
systemctl is-enabled s-ui > "$backup_dir/enabled-state.txt" || true
systemctl cat s-ui > "$backup_dir/systemd-effective.txt"
systemctl stop s-ui
if systemctl is-active --quiet s-ui; then echo 'Service still active' >&2; exit 1; fi
backup_paths=(usr/local/s-ui)
for path in etc/s-ui etc/systemd/system/s-ui.service etc/systemd/system/s-ui.service.d usr/bin/s-ui; do
  if test -e "/$path"; then backup_paths+=("$path"); fi
done
tar --acls --xattrs --numeric-owner -C / -czpf "$backup_dir/installation.tar.gz" "${backup_paths[@]}"
(cd "$backup_dir" && sha256sum installation.tar.gz > installation.tar.gz.sha256 && sha256sum -c installation.tar.gz.sha256)
tar -tzf "$backup_dir/installation.tar.gz" > "$backup_dir/inventory.txt"
```

Полная копия остановленного DB-каталога сохраняет SQLite WAL/SHM вместе с БД;
не копировать только живой `.db`. Проверить наличие бинарника, базы, ключей и
systemd в inventory и перенести backup вне машины. Если backup не получился,
не продолжать замену, восстановить исходное состояние службы.

```bash
install -m 0755 "$release_stage/s-ui/sui" /usr/local/s-ui/sui.new
mv -f /usr/local/s-ui/sui.new /usr/local/s-ui/sui
# Существующие unit, drop-ins, ключи, база и сертификаты сохраняются.
systemctl start s-ui
systemctl is-active --quiet s-ui
/usr/local/s-ui/sui -v
```

Startup выполняет миграции с environment действующей службы. Не запускать
`setting -reset`, `admin -reset` или чистую установку при обновлении. Если служба
раньше была выключена намеренно, провести проверку в согласованное окно и вернуть
исходное active/enabled состояние. Проверить вход, JSON-подписку, Raw Links/QR,
SNI и подключение Shadowrocket. Не присылать полные ссылки/пароли в отчётах.

При неудаче остановить службу, сохранить неудачную установку отдельно, проверить
backup checksum, вернуть **одновременно** старые бинарник и базу вместе с ключами
и unit/drop-ins. Для стандартных путей, пока служба остановлена:

```bash
systemctl stop s-ui
(cd "$backup_dir" && sha256sum --check --strict installation.tar.gz.sha256)
mv /usr/local/s-ui "/usr/local/s-ui.failed.$(date -u +%Y%m%dT%H%M%SZ)"
tar --acls --xattrs --numeric-owner -C / -xzpf "$backup_dir/installation.tar.gz"
systemctl daemon-reload
# Запустить только если исходная служба была active; вернуть прежний enabled state.
systemctl start s-ui
systemctl is-active --quiet s-ui
```

Эта процедура обновления не меняет unit/drop-ins; при ручном изменении их во время
эксперимента сначала убрать добавленные файлы, которых не было в backup.
Не делать downgrade только бинарника после миграции БД.

## Последовательность публикации (пока не выполнялась)

1. Review и обязательные CI PR подготовки; ручное объединение в основную ветку.
2. Зафиксировать полный SHA основной ветки, проверить `config/version`, release
   notes, свободу `v1.1.2-tt.1` в обоих репозиториях и отсутствие Release. Если
   upstream уже занял тег, выбрать новый и согласованно обновить метаданные.
3. Выполнить pre-tag gate и Test Build на этом SHA, Debian 12/Shadowrocket smoke.
   Проверить необходимые GitHub permissions; не подменять проверки их отключением.
4. Только после отдельного разрешения на публикацию создать annotated tag на
   сохранённом SHA и push **только этого тега**, без force и без `--tags`.
   Тег запускает существующий Release, Windows и Docker workflow. Не создавать
   Release вручную: draft/assets/classification уже реализованы в `release.yml`.
5. Дождаться всех gates, 27 assets, `isPrerelease=true`, `latest=false`, SBOM,
   attestations и совпадения source SHA. Нельзя считать успех Docker доказательством
   успеха Linux Release или наоборот. Не заменять опубликованные assets.
6. Вручную запустить существующий Release Smoke для этого тега. Повторить Debian 12
   установку/обновление на тестовой VM и Shadowrocket import/connect; только затем
   рекомендовать обновление действующих систем с резервной копией.
