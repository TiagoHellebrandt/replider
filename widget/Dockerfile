# ── widget/Dockerfile ────────────────────────────────────────────────────────
# Builds the embeddable webchat widget and packages it as a tiny static site
# served by nginx. Two-stage so we don't ship Node into production.

# Stage 1 — build
FROM node:22-alpine AS build
WORKDIR /app

# Cache deps first
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY tsconfig.json vite.config.ts ./
COPY src ./src
RUN npm run build

# Stage 2 — serve
FROM nginx:1.27-alpine
RUN rm -rf /usr/share/nginx/html/*

# The Vite IIFE build emits widget.js + widget.css (vite 8 names the lib CSS
# asset after the entry — it was style.css under vite 5). We expose them at
# predictable URLs so customer sites can hot-link them.
COPY --from=build /app/dist/widget.js     /usr/share/nginx/html/widget.js
COPY --from=build /app/dist/widget.js.map /usr/share/nginx/html/widget.js.map
COPY --from=build /app/dist/widget.css    /usr/share/nginx/html/widget.css

COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
