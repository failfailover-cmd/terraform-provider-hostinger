# Terraform Provider for Hostinger

Terraform provider для управления веб-сайтами на Hostinger Hosting через API.

## Возможности

- ✅ Создание веб-сайтов на Hostinger
- ✅ Удаление веб-сайтов
- ✅ Получение списка хостинг-планов (orders)
- ✅ Получение списка существующих сайтов
- ✅ Поддержка приватного GitLab Package Registry

## Требования

- Terraform >= 1.0
- Go >= 1.21 (для разработки)
- Hostinger API Token

## Установка

### Локальная установка (для разработки)

```bash
# Собрать провайдер
make build

# Установить локально
make install
```

### Использование из GitLab Package Registry

```hcl
terraform {
  required_providers {
    hostinger = {
      source  = "gitlab.com/yourusername/terraform-provider-hostinger"
      version = "~> 1.0"
    }
  }
}
```

## Использование

### Конфигурация провайдера

```hcl
provider "hostinger" {
  api_token = var.hostinger_api_token  # или используйте HOSTINGER_API_TOKEN env var
}
```

### Создание веб-сайта

```hcl
resource "hostinger_website" "example" {
  domain   = "example.com"
  order_id = 1234567
}
```

### Создание первого сайта на новом хостинге

```hcl
resource "hostinger_website" "first_site" {
  domain          = "example.com"
  order_id        = 1234567
  datacenter_code = "us-east-1"  # Обязательно для первого сайта
}
```

### Data Source: Список заказов

```hcl
data "hostinger_orders" "all" {}

output "orders" {
  value = data.hostinger_orders.all.orders
}
```

### Массовое создание сайтов

```hcl
locals {
  domains = [
    "site1.com",
    "site2.com",
    "site3.com",
  ]
}

resource "hostinger_website" "sites" {
  for_each = toset(local.domains)
  
  domain   = each.value
  order_id = var.hostinger_order_id
}
```

## Разработка

### Структура проекта

```
terraform-provider-hostinger/
├── main.go                      # Точка входа
├── internal/
│   └── provider/
│       ├── provider.go          # Конфигурация провайдера
│       ├── resource_website.go  # Ресурс website
│       ├── data_source_orders.go # Data source orders
│       └── client/
│           └── client.go        # HTTP клиент для Hostinger API
├── examples/                    # Примеры использования
├── docs/                        # Документация
├── .gitlab-ci.yml              # GitLab CI/CD
├── Makefile                     # Команды для сборки
└── go.mod
```

### Команды разработки

```bash
# Установить зависимости
go mod download

# Запустить тесты
make test

# Собрать провайдер
make build

# Установить локально
make install

# Создать релиз
make release
```

## Получение API токена

1. Перейдите на https://hpanel.hostinger.com/account/api
2. Создайте новый API токен
3. Скопируйте токен

## Переменные окружения

- `HOSTINGER_API_TOKEN` - API токен Hostinger (альтернатива указанию в конфигурации)

## Лицензия

MIT License

## Автор

Создано для управления Hostinger хостингом через Terraform
