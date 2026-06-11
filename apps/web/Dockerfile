# syntax=docker/dockerfile:1
FROM node:20-alpine AS builder

WORKDIR /build
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/web/package.json apps/web/
COPY packages/types/package.json packages/types/
COPY packages/tsconfig/package.json packages/tsconfig/
RUN corepack enable && pnpm install --frozen-lockfile

COPY . .
RUN corepack enable && pnpm --filter @tpt/web build

FROM nginx:alpine

COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /build/apps/web/dist /usr/share/nginx/html

EXPOSE 80

ENTRYPOINT ["nginx", "-g", "daemon off;"]