FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ops-admin .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/uploads

WORKDIR /app
COPY --from=builder /out/ops-admin /app/ops-admin

EXPOSE 8082
CMD ["/app/ops-admin"]
