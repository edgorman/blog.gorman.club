# Multi-stage build producing a static Nginx image. Every VITE_* value is baked in at build
# time, same as the Cloudflare Pages build (see push-commit.yaml / promote-release.yaml).
FROM node:22-alpine AS build
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

ARG VITE_BACKEND_URL=""
ENV VITE_BACKEND_URL=$VITE_BACKEND_URL

# Firebase Auth config - the app uses Firebase only to mint the ID token the backend verifies,
# never to reach Firestore. The API key is not a secret for web apps (see src/lib/firebase.ts).
ARG VITE_FIREBASE_API_KEY=""
ARG VITE_FIREBASE_AUTH_DOMAIN=""
ARG VITE_FIREBASE_PROJECT_ID=""
ENV VITE_FIREBASE_API_KEY=$VITE_FIREBASE_API_KEY
ENV VITE_FIREBASE_AUTH_DOMAIN=$VITE_FIREBASE_AUTH_DOMAIN
ENV VITE_FIREBASE_PROJECT_ID=$VITE_FIREBASE_PROJECT_ID

RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /src/dist /usr/share/nginx/html

EXPOSE 80
