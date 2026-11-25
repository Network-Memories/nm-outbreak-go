FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/nm-outbreak ./cmd/nm-outbreak

FROM alpine:3.21
RUN adduser -D -u 10001 nm && mkdir -p /var/lib/nm-outbreak && chown nm:nm /var/lib/nm-outbreak
WORKDIR /var/lib/nm-outbreak
ENV NM_ACCOUNT_FILE=/var/lib/nm-outbreak/accounts.json
VOLUME ["/var/lib/nm-outbreak"]
USER nm
COPY --from=builder /out/nm-outbreak /usr/local/bin/nm-outbreak
ENTRYPOINT ["/usr/local/bin/nm-outbreak"]

COPY THIRD_PARTY_NOTICES.md /usr/share/licenses/nm-outbreak/THIRD_PARTY_NOTICES.md
COPY licenses /usr/share/licenses/nm-outbreak/
