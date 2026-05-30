# --- STAGE 1: Compilation Environment ---
FROM golang:1.22-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy dependency architecture maps first to utilize caching
COPY go.mod ./
RUN go mod download

# Copy the rest of the source files (including index.html)
COPY . .

# Compile the native Go binary with network optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fshare-engine .

# --- STAGE 2: Microscopic Production Environment ---
FROM alpine:3.19

WORKDIR /root/

# Install root SSL certificates so Go can execute secure HTTPS API calls
RUN apk --no-cache add ca-certificates

# Copy only the compiled binary from the builder layer
COPY --from=builder /app/fshare-engine .

# Expose your Stremio addon port
EXPOSE 8787

# Command to execute the server application
CMD ["./fshare-engine"]
