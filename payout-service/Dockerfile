FROM golang:1.25-alpine AS build
WORKDIR /src
COPY reuse/xs_wallet ./reuse/xs_wallet
COPY payout-service/go.mod payout-service/go.sum ./payout-service/
WORKDIR /src/payout-service
RUN go mod download
COPY payout-service . .
RUN CGO_ENABLED=0 go build -o /out/payoutd ./cmd/payoutd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/payoutd /payoutd
ENTRYPOINT ["/payoutd"]
