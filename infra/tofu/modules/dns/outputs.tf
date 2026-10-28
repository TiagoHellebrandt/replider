output "managed_hostnames" {
  description = "Every hostname this module owns — the blast radius of a cutover."
  value = concat(
    [var.domain, "www.${var.domain}"],
    [for name, _ in var.subdomains : "${name}.${var.domain}"]
  )
}

output "origin_ipv4" {
  description = "Current origin. Change the variable, apply, and traffic follows."
  value       = var.origin_ipv4
}
