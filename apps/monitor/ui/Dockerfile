# syntax=docker/dockerfile:1

# monorepo (bun workspaces) 上の TanStack Start (Vite) ビルド。
# ビルドコンテキストはリポジトリルート:  docker build -f apps/monitor/ui/Dockerfile .

# ---- deps: lockfile + 全ワークスペースの package.json だけで依存解決 ----
FROM oven/bun:1.3.13-alpine AS deps
WORKDIR /repo
COPY package.json bun.lock ./
COPY apps/web/package.json apps/web/package.json
COPY apps/monitor/ui/package.json apps/monitor/ui/package.json
# The root package.json lists every workspace; bun refuses to install with any
# of them missing ("Workspace not found"), even ones monitor does not use.
COPY apps/desktop/package.json apps/desktop/package.json
COPY packages/agent-antigravity-cli/package.json packages/agent-antigravity-cli/package.json
COPY packages/agent-codex/package.json packages/agent-codex/package.json
COPY packages/agent-runtime/package.json packages/agent-runtime/package.json
COPY packages/desktop-bridge/package.json packages/desktop-bridge/package.json
COPY packages/graph-domain/package.json packages/graph-domain/package.json
# apps/web declares "@keyhole-koro/paper-in-paper": "file:./vender/paper-in-paper"
# (a submodule). monitor does not use it, but it shares the workspace lockfile,
# so bun still has to link it — and a file: dependency cannot be linked from a
# package.json alone. Without the directory here, the install fails with
# "ENOENT: ... failed to link package: @keyhole-koro/paper-in-paper".
COPY apps/web/vender/paper-in-paper apps/web/vender/paper-in-paper
RUN bun install --frozen-lockfile

# ---- prod-deps: monitor の実行時依存だけ (devDependencies と他ワークスペースを除く) ----
FROM deps AS prod-deps
RUN rm -rf node_modules apps/*/node_modules apps/*/*/node_modules packages/*/node_modules \
 && bun install --frozen-lockfile --production --filter @synthify/monitor-ui

# ---- builder: ソースを載せて monitor をビルド (dist/client + dist/server) ----
FROM oven/bun:1.3.13-alpine AS builder
WORKDIR /repo
COPY --from=deps /repo/node_modules ./node_modules
# bun installs a workspace member's own dependencies under that member, not at
# the root: `vite` and its bin symlink live in apps/monitor/ui/node_modules/.bin,
# and the root node_modules/.bin does not carry them. Copying only the root tree
# leaves `bun run build` with "vite: not found" (exit 127).
COPY --from=deps /repo/apps/monitor/ui/node_modules ./apps/monitor/ui/node_modules
COPY . .

# NEXT_PUBLIC_* はクライアントバンドルへビルド時に焼き込まれるので build-arg で渡す
# (Next.js 時代の名前のまま。vite.config.ts の envPrefix で読ませている)。
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
    NEXT_PUBLIC_FIREBASE_APP_ID=$NEXT_PUBLIC_FIREBASE_APP_ID

WORKDIR /repo/apps/monitor/ui
RUN bun run build

# ---- runner: ビルド結果と実行時の依存だけを載せる ----
# サーバーバンドル (dist/server) は react / recharts / firebase-admin / pg などを
# 実行時に node_modules から読むので、monitor の本番依存だけを同じ配置で載せる。
FROM oven/bun:1.3.13-alpine AS runner
WORKDIR /repo
ENV NODE_ENV=production
# PORT は Cloud Run が注入する (server.ts が process.env.PORT を読む)。
COPY --from=prod-deps /repo/node_modules ./node_modules
COPY --from=prod-deps /repo/apps/monitor/ui/node_modules ./apps/monitor/ui/node_modules
COPY --from=builder /repo/apps/monitor/ui/package.json ./apps/monitor/ui/package.json
COPY --from=builder /repo/apps/monitor/ui/server.ts ./apps/monitor/ui/server.ts
COPY --from=builder /repo/apps/monitor/ui/dist ./apps/monitor/ui/dist

WORKDIR /repo/apps/monitor/ui
EXPOSE 8080
CMD ["bun", "server.ts"]
