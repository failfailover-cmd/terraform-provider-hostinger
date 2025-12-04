# Инструкция по настройке GitLab для публикации провайдера

## 1. Добавить GPG ключ в GitLab CI/CD

Перейдите в ваш проект GitLab:
https://gitlab.com/a4765/infra/devops/terraform_providers/hostinger/-/settings/ci_cd

### Шаг 1: Раскрыть секцию "Variables"

Нажмите **Expand** рядом с "Variables".

### Шаг 2: Добавить переменную GPG_PRIVATE_KEY

- Нажмите **Add variable**
- **Key**: `GPG_PRIVATE_KEY`
- **Value**: Скопируйте содержимое файла `/tmp/gpg_private_key.asc` (весь текст от BEGIN до END)
- **Type**: Variable
- **Flags**: 
  - ✅ Protect variable (чтобы использовалась только в protected branches)
  - ✅ Mask variable (чтобы не показывалась в логах)
- Нажмите **Add variable**

### Шаг 3: Добавить переменную GPG_FINGERPRINT

- Нажмите **Add variable**
- **Key**: `GPG_FINGERPRINT`
- **Value**: `3BD36CCD800C3CEEEC0176F72D97EC5025C2EB60`
- **Type**: Variable
- **Flags**: 
  - ✅ Protect variable
- Нажмите **Add variable**

## 2. Создать первый релиз

После настройки переменных, создайте тег для первого релиза:

```bash
cd /opt/scripts/hostinger_gitlab
git tag v1.0.0
git push origin v1.0.0
```

GitLab CI автоматически:
1. Соберет провайдер для Linux, Windows, macOS (amd64 и arm64)
2. Подпишет релиз GPG ключом
3. Опубликует в GitLab Releases

## 3. Проверить релиз

После завершения пайплайна (обычно 2-3 минуты), проверьте:
https://gitlab.com/a4765/infra/devops/terraform_providers/hostinger/-/releases

Вы должны увидеть релиз v1.0.0 с бинарниками для всех платформ.

## 4. Использование провайдера коллегами

Коллеги должны:

### Шаг 1: Создать Personal Access Token в GitLab

1. Перейти: https://gitlab.com/-/user_settings/personal_access_tokens
2. Создать токен с правами: `read_api`
3. Скопировать токен

### Шаг 2: Настроить Terraform CLI

Создать/отредактировать файл `~/.terraformrc` (Linux/Mac) или `%APPDATA%\terraform.rc` (Windows):

```hcl
credentials "gitlab.com" {
  token = "ваш-личный-токен"
}
```

### Шаг 3: Использовать провайдер

В вашем `main.tf`:

```hcl
terraform {
  required_providers {
    hostinger = {
      source  = "gitlab.com/a4765/hostinger"
      version = "~> 1.0"
    }
  }
}

provider "hostinger" {
  # Токен через переменную окружения HOSTINGER_API_TOKEN
}

resource "hostinger_website" "example" {
  domain   = "example.com"
  order_id = 1006933104
}
```

Затем:

```bash
terraform init
terraform plan
terraform apply
```

## Готово! 🎉

Ваш провайдер теперь доступен для всей команды через GitLab Package Registry.
