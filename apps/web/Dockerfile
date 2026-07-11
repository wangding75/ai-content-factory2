FROM node:24-alpine AS dependencies
WORKDIR /app
RUN corepack enable
COPY package.json ./
RUN pnpm install --no-frozen-lockfile --config.node-linker=hoisted

FROM node:24-alpine AS build
WORKDIR /app
ARG API_BASE_URL=http://api:8080/api/v1
ENV API_BASE_URL=$API_BASE_URL
RUN corepack enable
COPY --from=dependencies /app/node_modules ./node_modules
COPY . .
RUN pnpm build

FROM node:24-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
RUN corepack enable
COPY --from=build /app ./
EXPOSE 3000
CMD ["pnpm", "start"]
