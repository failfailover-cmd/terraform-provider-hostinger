data "hostinger_websites" "all" {}

output "websites" {
  value = data.hostinger_websites.all.websites
}
