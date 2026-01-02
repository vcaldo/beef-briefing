# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

# Copy package files
COPY apps/deck-mini-app/package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY apps/deck-mini-app/ .

# Build argument for API URL
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL

# Build the application
RUN npm run build

# Production stage
FROM nginx:alpine

# Copy custom nginx config
COPY apps/deck-mini-app/nginx.conf /etc/nginx/conf.d/default.conf

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# Expose port
EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
