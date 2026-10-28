# ── app/Dockerfile ────────────────────────────────────────────────────────────
# The Flutter web app is pre-built by GitHub Actions before docker build is called.
# Context: app/   Expected: build/web/ folder from `flutter build web --release`
FROM nginx:1.27-alpine

# Remove the default nginx page
RUN rm -rf /usr/share/nginx/html/*

# Copy the pre-built Flutter web output
COPY build/web/ /usr/share/nginx/html/

# Use our custom nginx config (Flutter SPA needs index.html fallback)
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
