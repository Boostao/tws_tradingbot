FROM node:20-alpine AS build

WORKDIR /app

ENV NODE_ENV=development

COPY package.json package-lock.json ./
RUN npm ci --include=dev

COPY . .
RUN npm run prepare
RUN npm run build

FROM node:20-alpine

WORKDIR /app

ENV NODE_ENV=production

COPY package.json package-lock.json ./
RUN npm ci --omit=dev

COPY --from=build /app/build /app/build
COPY --from=build /app/static /app/static

EXPOSE 5173

CMD ["node", "build"]
