variable "cloudflare_zone_id" {
  description = "Cloudflare zone id for replider.com."
  type        = string
}

variable "domain" {
  type    = string
  default = "replider.com"
}

variable "origin_ipv4" {
  description = "Current VPS public IPv4 (the migration switch)."
  type        = string
}

variable "ttl" {
  description = "TTL for non-proxied records."
  type        = number
  default     = 300
}
