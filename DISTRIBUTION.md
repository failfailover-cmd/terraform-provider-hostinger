# Публикация Hostinger Terraform Provider

Исходники и релизы: https://github.com/failfailover-cmd/terraform-provider-hostinger.
Terraform Registry source: `failfailover-cmd/hostinger`.

## Установка

```hcl
terraform {
  required_providers {
    hostinger = {
      source  = "failfailover-cmd/hostinger"
      version = "~> 1.0"
    }
  }
}

provider "hostinger" {}
```

API-токен Hostinger передаётся через `HOSTINGER_API_TOKEN` либо параметр
`api_token`. GitLab credentials для установки из публичного Terraform
Registry не нужны. После публикации новой версии существующие стеки
обновляют provider и lock-файл через `terraform init -upgrade`.

## Релиз

1. Создать PR в `main`; дождаться успешных Tests/build и слить изменения.
2. Убедиться, что выбранный тег `vX.Y.Z` свободен, а локальная `main`
   соответствует актуальной `origin/main` с принятым PR.
3. Создать тег на принятом коммите и запушить его в `origin`.
4. Workflow `.github/workflows/release.yml` собирает архивы через GoReleaser,
   подписывает SHA256SUMS и публикует GitHub Release. Версия Go берётся из
   `go.mod`, версия бинарника — из тега через `main.version`.
5. Проверить успешное завершение workflow, наличие архивов, SHA256SUMS и
   подписи в GitHub Release, затем доступность версии в Terraform Registry.

Секреты workflow уже настроены в GitHub Actions: `GPG_PRIVATE_KEY`,
`PASSPHRASE`; `GITHUB_TOKEN` выдаётся самому job. Не копировать их в репозиторий.
Публикация версии не обновляет уже запущенные Terraform jobs автоматически.

## Локальная проверка

```bash
env -u TF_ACC go test -race ./... -timeout=90s
go build -o terraform-provider-hostinger .
```

Эти проверки не создают сайты и не меняют инфраструктуру. Acceptance-тесты
с `TF_ACC` используют реальный API и требуют отдельного согласования.
