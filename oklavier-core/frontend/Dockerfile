FROM node:25-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ENV NEXT_PUBLIC_OKLAVIER_API=http://oklavier-api.oklavier.svc.cluster.local:8080
RUN npm run build

FROM node:25-alpine
WORKDIR /app
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
ENV NEXT_PUBLIC_OKLAVIER_API=http://oklavier-api.oklavier.svc.cluster.local:8080
ENV HOSTNAME=0.0.0.0
ENV PORT=3000
EXPOSE 3000
CMD ["node", "server.js"]
