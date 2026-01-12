# Build stage
FROM node:22-alpine AS builder

# Install pnpm which properly supports workspace: protocol
RUN npm install -g pnpm

WORKDIR /app

# Copy root and workspace package files
COPY package*.json pnpm-workspace.yaml ./
COPY apps/deck-mini-app/ ./apps/deck-mini-app/
COPY apps/shared-mini-app/ ./apps/shared-mini-app/

# Install dependencies
RUN pnpm install --frozen-lockfile || pnpm install

# Change to deck-mini-app directory for build
WORKDIR /app/apps/deck-mini-app

# Build arguments for API URLs and New Relic Browser monitoring
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL

ARG VITE_CARD_API_URL
ENV VITE_CARD_API_URL=$VITE_CARD_API_URL

ARG VITE_NEW_RELIC_BROWSER_ACCOUNT_ID
ENV VITE_NEW_RELIC_BROWSER_ACCOUNT_ID=$VITE_NEW_RELIC_BROWSER_ACCOUNT_ID

ARG VITE_NEW_RELIC_BROWSER_APP_ID
ENV VITE_NEW_RELIC_BROWSER_APP_ID=$VITE_NEW_RELIC_BROWSER_APP_ID

ARG VITE_NEW_RELIC_BROWSER_LICENSE_KEY
ENV VITE_NEW_RELIC_BROWSER_LICENSE_KEY=$VITE_NEW_RELIC_BROWSER_LICENSE_KEY

ARG VITE_ENVIRONMENT
ENV VITE_ENVIRONMENT=$VITE_ENVIRONMENT

# Build the application
RUN pnpm run build

# Production stage
FROM nginx:alpine

# Copy custom nginx config
COPY apps/deck-mini-app/nginx.conf /etc/nginx/conf.d/default.conf

# Copy built assets
COPY --from=builder /app/apps/deck-mini-app/dist /usr/share/nginx/html

# Expose port
EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
