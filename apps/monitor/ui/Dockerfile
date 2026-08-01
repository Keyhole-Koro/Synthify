# syntax=docker/dockerfile:1

# monorepo (bun workspaces) 上の Next.js standalone ビルド。
# ビルドコンテキストはリポジトリルート:  docker build -f apps/monitor/ui/Dockerfile .

# ---- deps: lockfile + 全ワークスペースの package.json だけで依存解決 ----
FROM oven/bun:1.3.13-alpine AS deps
WORKDIR /repo
COPY package.json bun.lock ./
COPY apps/web/package.json apps/web/package.json
COPY apps/monitor/ui/package.json apps/monitor/ui/package.json
# apps/web declares "@keyhole-koro/paper-in-paper": "file:./vender/paper-in-paper"
# (a submodule). monitor does not use it, but it shares the workspace lockfile,
# so bun still has to link it — and a file: dependency cannot be linked from a
# package.json alone. Without the directory here, the install fails with
# "ENOENT: ... failed to link package: @keyhole-koro/paper-in-paper".
COPY apps/web/vender/paper-in-paper apps/web/vender/paper-in-paper
RUN bun install --frozen-lockfile

# ---- builder: ソースを載せて monitor を standalone ビルド ----
FROM oven/bun:1.3.13-alpine AS builder
WORKDIR /repo
COPY --from=deps /repo/node_modules ./node_modules
# bun installs a workspace member's own dependencies under that member, not at
# the root: `next` and its bin symlink live in apps/monitor/ui/node_modules/.bin,
# and the root node_modules/.bin does not carry them. Copying only the root tree
# leaves `bun run build` with "next: not found" (exit 127).
COPY --from=deps /repo/apps/monitor/ui/node_modules ./apps/monitor/ui/node_modules
COPY . .

# NEXT_PUBLIC_* はクライアントバンドルへビルド時に焼き込まれるので build-arg で渡す。
ARG NEXT_PUBLIC_FIREBASE_API_KEY
ARG NEXT_PUBLIC_FIREBASE_PROJECT_ID
ARG NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN
ARG NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET
ARG NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID
ARG NEXT_PUBLIC_FIREBASE_APP_ID
ENV NEXT_PUBLIC_FIREBASE_API_KEY=$NEXT_PUBLIC_FIREBASE_API_KEY \
    NEXT_PUBLIC_FIREBASE_PROJECT_ID=$NEXT_PUBLIC_FIREBASE_PROJECT_ID \
    NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN=$NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN \
    NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET=$NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET \
    NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID=$NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID \
    NEXT_PUBLIC_FIREBASE_APP_ID=$NEXT_PUBLIC_FIREBASE_APP_ID \
    NEXT_TELEMETRY_DISABLED=1

WORKDIR /repo/apps/monitor/ui
RUN bun run build

# ---- runner: standalone 出力のみを載せた薄いイメージ ----
FROM oven/bun:1.3.13-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
# PORT は Cloud Run が注入する (Next standalone server は process.env.PORT を読む)。

# outputFileTracingRoot=リポジトリルートなので standalone はリポジトリ構造を保持する:
#   .next/standalone/apps/monitor/ui/server.js  +  .next/standalone/node_modules
COPY --from=builder /repo/apps/monitor/ui/.next/standalone ./
COPY --from=builder /repo/apps/monitor/ui/.next/static ./apps/monitor/ui/.next/static

EXPOSE 8080
CMD ["bun", "apps/monitor/ui/server.js"]
