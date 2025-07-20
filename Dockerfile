# Stage 1: Build Controller Frontend
FROM node:24-alpine AS controller-builder

RUN npm install pnpm@10 -g

WORKDIR /app/sdk
COPY sdk/package.json sdk/pnpm-lock.yaml* ./
RUN pnpm install
COPY sdk/ ./
RUN pnpm run build

WORKDIR /app/server/controller
COPY server/controller/package.json server/controller/pnpm-lock.yaml* ./
RUN pnpm install
COPY server/controller/ ./
RUN pnpm build

# Stage 2: Build Go Backend
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app
# Copy Go module files
COPY server/go.mod server/go.sum ./
# Download dependencies
RUN go mod download
# Copy the entire server source
COPY server/ ./
# Copy built frontend assets from previous stages
COPY --from=controller-builder /app/server/controller/dist ./controller/dist
# Build the Go application, embedding the frontend assets
RUN go build -o /controly .

# Stage 3: Final production image
FROM alpine:3.21
WORKDIR /app
# Copy the final executable from the backend-builder stage
COPY --from=backend-builder /controly .
# Expose the default port the application runs on
EXPOSE 8080
# Set default port for the server
ENV CONTROLY_PORT=8080
# Run the application
CMD ["/app/controly"]
