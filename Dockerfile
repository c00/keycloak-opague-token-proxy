# Use the official Go image as the base image
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache for dependencies
COPY go.mod go.sum ./

# Download Go dependencies into the container
RUN go mod download

# Copy the Go project files to the container
COPY . .

# Build the Go project
RUN go build -o bin/server main.go

## Here create the new container that will server as the runner. Copy in the build output.
FROM alpine:3.21

# Set the working directory inside the container
WORKDIR /app

# Copy the built binary from the builder container
COPY --from=builder /app/bin/server .

# Create a user to run the binary
RUN adduser -D appuser

USER appuser

# Set the entry point to run the built binary
CMD ["./server"]
