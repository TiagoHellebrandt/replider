# Production environment (US-F014).
#
# Scope is deliberately DNS-only for now. The VPS itself and the compose
# stack stay hand-managed until the migration target is decided
# (docs/infra-as-code.md: DigitalOcean DOKS after validation) — codifying a
# host we intend to leave buys nothing. What we DO want in code today is the
# switch that performs the leaving.

terraform {
  required_version = ">= 1.6"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.40"
    }
  }

  # Remote state with locking. Local state on a laptop is a single point of
  # failure for the thing whose job is to survive single points of failure.
  # SECRETS NEVER GO IN STATE: this config holds no passwords, and the
  # Cloudflare token comes from the environment (CLOUDFLARE_API_TOKEN).
  backend "gcs" {
    bucket = "replider-tofu-state"
    prefix = "prod-hostinger"
  }
}

provider "cloudflare" {
  # Reads CLOUDFLARE_API_TOKEN from the environment. Scope the token to
  # Zone:DNS:Edit on this zone only — an IaC token that can also purge cache
  # or edit firewall rules is a bigger blast radius than the job needs.
}

module "dns" {
  source = "../../modules/dns"

  cloudflare_zone_id = var.cloudflare_zone_id
  domain             = var.domain
  origin_ipv4        = var.origin_ipv4
  ttl                = var.ttl
}

output "managed_hostnames" {
  value = module.dns.managed_hostnames
}
