resource "hostinger_website" "example" {
  domain   = "example.com"
  order_id = 1234567
}

resource "hostinger_website" "first_site" {
  domain          = "first-site.com"
  order_id        = 1234567
  datacenter_code = "us-east-1"
}
