# ── web/Dockerfile ────────────────────────────────────────────────────────────
# The Astro site is pre-built by GitHub Actions before docker build is called.
# Context: web/   Expected: dist/ folder from `npm run build`
FROM nginx:1.27-alpine

# Remove the default nginx page
RUN rm -rf /usr/share/nginx/html/*

# Copy the pre-built Astro static output
COPY dist/ /usr/share/nginx/html/

# Use our custom nginx config (SPA-friendly, caching headers, gzip)
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
