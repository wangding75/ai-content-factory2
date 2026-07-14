FROM node:24-alpine AS base
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

FROM base AS dependencies
WORKDIR /app
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY apps/web/package.json ./apps/web/
# Install dependencies strictly using frozen-lockfile
RUN pnpm install --frozen-lockfile

FROM base AS build
WORKDIR /app
COPY --from=dependencies /app/node_modules ./node_modules
COPY --from=dependencies /app/apps/web/node_modules ./apps/web/node_modules
COPY . .

# Set API_BASE_URL for Next.js build-time environment
ARG API_BASE_URL=http://api:8080/api/v1
ENV API_BASE_URL=$API_BASE_URL

RUN pnpm --dir apps/web build

FROM base AS runtime
WORKDIR /app
ENV NODE_ENV=production

# Copy necessary files for running the workspace next app
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/apps/web/node_modules ./apps/web/node_modules
COPY --from=build /app/package.json ./package.json
COPY --from=build /app/pnpm-workspace.yaml ./pnpm-workspace.yaml
COPY --from=build /app/apps/web ./apps/web

EXPOSE 3000
WORKDIR /app/apps/web

# Start Next.js using pnpm start (next start)
CMD ["pnpm", "start"]
