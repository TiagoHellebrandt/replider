terraform {
  required_version = ">= 1.6"
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.40"
    }
  }
}

# Apex. Cloudflare flattens the apex A record for us; the origin IP is the
# same single variable every subdomain uses, so a migration touches one line.
resource "cloudflare_record" "apex" {
  zone_id = var.cloudflare_zone_id
  name    = "@"
  type    = "A"
  content = var.origin_ipv4
  ttl     = 1 # 1 = "automatic", required by Cloudflare when proxied = true
  proxied = true
  comment = "Marketing site (managed by OpenTofu — US-F014)"
}

resource "cloudflare_record" "www" {
  zone_id = var.cloudflare_zone_id
  name    = "www"
  type    = "CNAME"
  content = var.domain
  ttl     = 1
  proxied = true
  comment = "www -> apex (managed by OpenTofu)"
}

resource "cloudflare_record" "subdomain" {
  for_each = var.subdomains

  zone_id = var.cloudflare_zone_id
  name    = each.key
  type    = "A"
  content = var.origin_ipv4
  # Proxied records must use ttl = 1; only grey-clouded ones honour a real
  # TTL, and those are exactly the ones where a low TTL buys a fast cutover.
  ttl     = each.value.proxied ? 1 : var.ttl
  proxied = each.value.proxied
  comment = "${each.value.comment} (managed by OpenTofu)"
}
