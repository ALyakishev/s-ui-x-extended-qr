# Синхронизация upstream через PR

Workflow `.github/workflows/upstream-sync.yml` запускается ежедневно в **06:00 UTC**
(09:00 по Москве) и вручную. GitHub может задерживать scheduled runs.
Запускайте вручную только из `s-ui-x-extended`: другие refs и другие репозитории
отсекаются до выдачи доступа к секрету.

Источник — `deposist/s-ui-x-extended`. Его default branch проверена через
`git ls-remote --symref ... HEAD`: сейчас это `s-ui-x-extended`. Workflow каждый
раз определяет её заново, а не полагается на имя `main`.

## Обязательная настройка GitHub

1. В **Settings → Environments** создайте environment **`upstream-sync`**.
   В **Deployment branches and tags → Selected branches and tags** разрешите
   **только branch `s-ui-x-extended`**, без разрешения тегов, wildcard и PR refs.
   Не выбирайте «No restriction» или все protected branches. Это обязательная
   граница безопасности: workflows из ещё не проверенного sync-PR не должны
   получить токен, даже если upstream изменит файлы GitHub Actions.
2. Создайте fine-grained PAT, желательно отдельной учётной записи автоматизации
   с write-доступом к этому форку. Resource owner — `ALyakishev`, Repository access
   — **только `s-ui-x-extended-qr`**. Repository permissions:
   **Contents: Read and write**, **Pull requests: Read and write**,
   **Workflows: Read and write**. Последнее необходимо для push коммитов upstream,
   меняющих `.github/workflows`; без него такие обновления будут отклонены.
   Metadata: read выдаётся GitHub автоматически. Других прав не требуется.
   Установите срок действия и предусмотрите ротацию/одобрение токена, если owner
   этого требует.
3. Добавьте PAT как **environment secret `UPSTREAM_SYNC_TOKEN`** в environment
   `upstream-sync`, **не как общий repository/organization secret**. Не помещайте
   значение в YAML, git config, URL или PR. Не используйте environment required
   reviewers, если нужна ежедневная работа без ручного подтверждения каждого
   запуска; branch restriction из пункта 1 при этом обязательна.
4. В **Settings → Actions → General** разрешите GitHub Actions и используемый
   first-party `actions/checkout`. Стандартный **Workflow permissions** можно
   оставить read-only: `GITHUB_TOKEN` workflow явно имеет только `contents: read`.
   Разрешение «Allow GitHub Actions to create and approve pull requests» не
   заменяет PAT и не решает ограничение запуска CI стандартным `GITHUB_TOKEN`.
   Для этого workflow включать эту опцию не требуется: PR создаёт PAT.
5. Существующие CI/Audit/Semgrep workflows должны быть включены. PAT создаёт
   обычные события `pull_request: opened/synchronize` и push, поэтому их
   `pull_request` triggers выполняются без изменения CI-workflows. Нельзя заменять
   PAT на `${{ github.token }}`: GitHub подавляет большинство событий от него.
   Если для запуска требуется одобрение GitHub policy или исчерпана квота Actions,
   это нужно исправить в Settings; workflow не обходит такие ограничения.
6. Защитите основную ветку ruleset/branch protection: требовать PR,
   review и успешные существующие CI-checks. Не давать PAT bypass-прав.
   Workflow никогда не вызывает merge PR, auto-merge, создание Release или тега.
   Для полного запрета автоматического слияния отключите **Allow auto-merge**
   в Settings → General → Pull Requests. Если на существующем sync-PR уже
   включён auto-merge, workflow откажется обновлять ветку до его отключения.

Для использования нового workflow он должен быть объединён в default branch
самим сопровождающим. На этапе установки это не происходит автоматически.

## Поведение

- Если upstream tip уже является предком основной ветки — успешный no-op,
  без изменения branch/PR и без обращения к PR API.
  Если предыдущий sync-PR объединён через squash, дополнительно сравниваются
  результирующие деревья: одинаковое содержимое не создаёт пустой PR каждый день.
- При обновлениях обычный Git merge создаёт/дополняет **`sync/upstream`**.
  Сначала учитывается актуальная основная ветка форка, затем upstream.
  Существующие коммиты sync-ветки, собственные TrustTunnel/tt:// и frontend-
  изменения не заменяются копией upstream. Совместимость всё равно требует CI
  и человеческого review; отсутствие текстового конфликта не гарантирует её.
- Обновляется один открытый PR `sync/upstream → s-ui-x-extended`. Повторный запуск
  без новых коммитов не создаёт дубликат или лишний merge-коммит. PR из чужого
  форка с таким же именем ветки игнорируется. После сбоя API создания PR
  следующий запуск использует уже опубликованную sync-ветку.
- Force-push не используется ни для одной ветки. Изменение удалённой ветки
  параллельно запуску может отклонить обычный push — это безопасная остановка.
- При конфликте merge отменяется, workflow завершается ошибкой и перечисляет
  конфликтующие файлы в job log и **Job summary**. Удалённая sync-ветка и PR
  не меняются. Вручную разрешите конфликт с сохранением нужных fork-изменений
  либо в существующей sync-ветке, либо в отдельно проверенной ветке. Стратегии
  `ours`/`theirs`, overwrite файлов и автоматическое разрешение не применяются.
- Upstream-код не исполняется в sync-job: нет npm/Go install, сборок, тестов,
  submodule update или git hooks. Токен доступен только одному sync-шагу.
- Отсутствующий/истёкший PAT или недостаточные права дают ошибку; fallback на
  `GITHUB_TOKEN` запрещён. Secret не нужен для обычных CI-проверок sync-PR.

## Проверки

`python3 -B .github/scripts/test_upstream_sync.py -v` использует локальные bare
Git-репозитории и заглушку только PR API. Проверяются no-op, default branch с
другим именем, сохранение fork/manual-коммитов, обновление основной ветки,
идемпотентность, конфликты до и после создания PR, повтор после сбоя API,
защита от дубликатов, squash-merge, чужой head с тем же именем, запрет auto-merge
и отсутствие Git-операций без PAT. Эти offline-тесты также выполняются в sync-job
перед шагом, которому передаётся токен.

Для end-to-end проверки после настройки environment запустите workflow вручную
из основной ветки. При наличии реальных обновлений проверьте открытие PR и
появление CI/Audit/Semgrep checks; при отсутствии обновлений ожидается no-op.
Локальные проверки не доказывают доступность токена или выполнение GitHub CI.

Справка GitHub: [события от токенов](https://docs.github.com/en/actions/how-tos/writing-workflows/choosing-when-your-workflow-runs/triggering-a-workflow),
[защита environment](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments),
[права PAT](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens).
