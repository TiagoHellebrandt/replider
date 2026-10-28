# DNS module (US-F014).
#
# DNS is the migration switch. Everything else about moving to another host
# is rehearsable; the cutover itself is one record change, and how fast it
# propagates is decided by a TTL you set BEFORE the incident. Managing it in
# code is what makes that TTL an auditable decision instead of a click
# somebody made in 2024.

variable "cloudflare_zone_id" {
  description = "Cloudflare zone id for the managed domain (Overview page, right column)."
  type        = string
}

variable "domain" {
  description = "Apex domain, e.g. replider.com."
  type        = string
}

variable "origin_ipv4" {
  description = <<-EOT
    Public IPv4 of the origin host (the VPS today). THE migration switch:
    change this one value, apply, and traffic follows within the TTL.
  EOT
  type        = string

  validation {
    condition     = can(regex("^\\d{1,3}(\\.\\d{1,3}){3}$", var.origin_ipv4))
    error_message = "origin_ipv4 must be a bare IPv4 address."
  }
}

variable "ttl" {
  description = <<-EOT
    TTL in seconds for NON-proxied records. Kept low (60-300) so a cutover
    is minutes, not hours. Note: Cloudflare IGNORES ttl on proxied records
    (they always resolve to Cloudflare's anycast IPs and the origin change
    takes effect immediately) — this matters for the grey-clouded ones.
  EOT
  type        = number
  default     = 300

  validation {
    condition     = var.ttl >= 60 && var.ttl <= 300
    error_message = "ttl must stay between 60 and 300 seconds — the whole point is a fast cutover."
  }
}

variable "subdomains" {
  description = <<-EOT
    Hostnames pointing at the origin. `proxied = true` puts Cloudflare in
    front (DDoS + TLS + hides the origin IP); false exposes the origin
    directly, which is required for anything doing its own ACME HTTP-01
    challenge or needing a non-HTTP port.
  EOT
  type = map(object({
    proxied = bool
    comment = string
  }))
  default = {
    "api"    = { proxied = true, comment = "Go API (nginx -> api container)" }
    "app"    = { proxied = true, comment = "Flutter dashboard" }
    "docs"   = { proxied = true, comment = "Redoc / OpenAPI spec" }
    "status" = { proxied = false, comment = "Uptime page — must NOT depend on our edge" }
  }
}
