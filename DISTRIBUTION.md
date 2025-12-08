# Как использовать Hostinger Terraform Provider

Этот провайдер размещен в нашем приватном GitLab Registry.

## 1. Настройка доступа (Один раз)

Чтобы Terraform мог скачивать провайдеры из нашего GitLab, нужно добавить токен доступа.

1. Создайте **Personal Access Token** в GitLab:
   - Scope: `read_api`
   - [Ссылка для создания](https://gitlab.com/-/profile/personal_access_tokens)

2. Создайте или отредактируйте файл `~/.terraformrc` (на Windows `%APPDATA%\terraform.rc`):

```hcl
credentials "gitlab.com" {
  token = "ваш-личный-токен"
}
```

## 2. Использование в проекте

В вашем `main.tf` укажите путь к провайдеру:

```hcl
terraform {
  required_providers {
    hostinger = {
      # Путь к провайдеру в GitLab
      source  = "gitlab.com/a4765/hostinger"
      version = "~> 1.0.0"
    }
  }
}

provider "hostinger" {
  # Токен можно передать через переменную окружения HOSTINGER_API_TOKEN
}

resource "hostinger_website" "example" {
  domain   = "example-internal.com"
  order_id = 1006933104
}
```

## 3. Установка

```bash
terraform init
```
