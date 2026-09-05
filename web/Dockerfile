FROM swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/node:20-alpine AS builder
COPY --from=oven/bun:1.4.2-alpine /usr/local/bin/bun /usr/local/bin/bun

WORKDIR /app
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

COPY . ./
RUN bun run build

FROM swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80
