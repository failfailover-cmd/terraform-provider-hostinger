terraform {
  required_providers {
    hostinger = {
      source  = "gitlab.com/yourusername/hostinger"
      version = "1.0.0"
    }
  }
}

provider "hostinger" {
  # API token will be read from HOSTINGER_API_TOKEN env var
}

# Получить список всех заказов
data "hostinger_orders" "all" {}

output "all_orders" {
  value = data.hostinger_orders.all.orders
}

# Получить список всех сайтов
data "hostinger_websites" "all" {}

output "all_websites" {
  value = data.hostinger_websites.all.websites
}

# Создать веб-сайт
resource "hostinger_website" "test_site" {
  domain   = "test-terraform-provider.com"
  order_id = 1006933104
}
