# Build stage: install the locked packages and build the standalone server.
FROM node:24-trixie-slim AS build
WORKDIR /src
ENV NEXT_TELEMETRY_DISABLED=1
# Copy the package files first, so Docker can keep the install layer when only the code changes.
COPY package.json package-lock.json .npmrc ./
RUN npm ci --no-audit --no-fund
COPY . .
# The build puts SITE_URL into the absolute URLs of link previews: docker build --build-arg SITE_URL=https://...
ARG SITE_URL=http://localhost:3000
ENV SITE_URL=$SITE_URL
RUN npm run build

# Run stage: only the standalone output, on a Node.js image with no shell, as a user that is not root.
FROM gcr.io/distroless/nodejs24-debian13:nonroot
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
COPY --from=build /src/.next/standalone ./
COPY --from=build /src/.next/static ./.next/static
EXPOSE 3000
CMD ["server.js"]
