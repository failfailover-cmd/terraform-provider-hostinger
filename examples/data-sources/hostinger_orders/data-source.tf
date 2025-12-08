data "hostinger_orders" "all" {}

output "orders" {
  value = data.hostinger_orders.all.orders
}
