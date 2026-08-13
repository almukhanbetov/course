FROM node:24-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

# Separate from `deps` deliberately: `deps` installs devDependencies too
# (typescript/eslint/@types/*, needed by `next build`'s own type-checking),
# but none of that belongs in the runtime image. This stage installs only
# what package.json's own dependencies (not devDependencies) list, so the
# final `run` stage below can copy a production-only node_modules instead
# of shipping build tooling nothing at runtime ever calls.
FROM node:24-alpine AS prod-deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev

FROM node:24-alpine AS build
WORKDIR /app
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:24-alpine AS run
WORKDIR /app
ENV NODE_ENV=production
# node:24-alpine already ships a low-privilege `node` user (uid 1000) — no
# need to create a new one.
COPY --from=build /app/public ./public
COPY --from=prod-deps /app/node_modules ./node_modules
COPY --from=build /app/package.json ./package.json
COPY --from=build --chown=node:node /app/.next ./.next
USER node
EXPOSE 3000
CMD ["npm", "run", "start"]
