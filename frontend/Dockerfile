# ─── Stage 1: Build ──────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

# Copy dependency manifests first for layer caching.
COPY package.json package-lock.json* ./
RUN npm ci --prefer-offline

# Copy source and build.
COPY . .

# VITE_API_BASE_URL is intentionally left empty so all API requests
# use relative paths (handled by nginx proxy_pass below).
ENV VITE_API_BASE_URL=""

RUN npm run build

# ─── Stage 2: Serve ───────────────────────────────────────────────────────────
FROM nginx:1.27-alpine

# Remove default nginx config.
RUN rm /etc/nginx/conf.d/default.conf

# Copy our hardened nginx config.
COPY nginx.conf /etc/nginx/conf.d/timesync.conf

# Copy the built SPA.
COPY --from=builder /app/dist /usr/share/nginx/html

# Run as non-root.
RUN chown -R nginx:nginx /usr/share/nginx/html && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /var/log/nginx && \
    touch /var/run/nginx.pid && \
    chown nginx:nginx /var/run/nginx.pid

USER nginx

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost/health || exit 1
