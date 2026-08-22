# Multi-stage build producing a static Nginx image. VITE_BACKEND_URL is baked in at build time,
# same as the Cloudflare Pages build (see push-commit.yaml / promote-release.yaml).
FROM node:22-alpine AS build
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

ARG VITE_BACKEND_URL=""
ENV VITE_BACKEND_URL=$VITE_BACKEND_URL
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /src/dist /usr/share/nginx/html

EXPOSE 80
