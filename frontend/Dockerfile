FROM node:22-alpine AS builder
ARG VITE_GATEWAY_URL=http://localhost:8080
ARG VITE_WS_URL=ws://localhost:8080/ws
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
ENV VITE_GATEWAY_URL=${VITE_GATEWAY_URL}
ENV VITE_WS_URL=${VITE_WS_URL}
RUN npm run build

FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
