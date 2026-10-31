# ---- Build stage: compile the Go program ----
FROM golang:1.27-alpine AS build

WORKDIR /src

# Download dependencies first, so Docker can reuse this layer
# when only our code changes.
COPY go.mod go.sum ./
RUN go mod download

# Copy the code and build the web server into a single binary.
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/soundcheck ./cmd/soundcheck

# ---- Run stage: a tiny image with just the binary ----
FROM alpine:3.20

# HTTPS certificates (for Ticketmaster) and time zone data.
RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /bin/soundcheck /soundcheck

CMD ["/soundcheck"]