# Stage 1: Build Controller Frontend
FROM node:23-alpine AS controller-builder
WORKDIR /app
COPY server/controller/package.json server/controller/pnpm-lock.yaml* ./
RUN npm install -g pnpm@10 && pnpm install
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
COPY --from=controller-builder /app/dist ./controller/dist
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
