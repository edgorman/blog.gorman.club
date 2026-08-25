# Multi-stage build producing a static Nginx image. Every VITE_* value is baked in at build
# time, same as the Cloudflare Pages build (see push-commit.yaml / promote-release.yaml).
FROM node:22-alpine AS build
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

ARG VITE_BACKEND_URL=""
ENV VITE_BACKEND_URL=$VITE_BACKEND_URL

# Google Sign-In client ID. Not a secret - it identifies the app, and the backend only accepts
# ID tokens minted for this same client ID (see src/hooks/useGoogleAuth.ts).
ARG VITE_GOOGLE_CLIENT_ID=""
ENV VITE_GOOGLE_CLIENT_ID=$VITE_GOOGLE_CLIENT_ID

RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /src/dist /usr/share/nginx/html

EXPOSE 80
