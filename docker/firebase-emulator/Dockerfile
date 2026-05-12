FROM eclipse-temurin:21-jre
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends curl \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*
RUN npm install -g firebase-tools@13
# Pre-download emulator binaries at build time so the container works without
# outbound network access at runtime.
RUN firebase setup:emulators:firestore && firebase setup:emulators:ui
WORKDIR /workspace
