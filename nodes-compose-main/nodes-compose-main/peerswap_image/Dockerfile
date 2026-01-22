FROM golang:1.18-buster as builder

ARG COMMIT

WORKDIR /app

RUN git clone https://github.com/ElementsProject/peerswap.git && cd peerswap && git checkout $COMMIT

# Build
RUN cd peerswap && make -j$(nproc) lnd-release

FROM debian:buster-slim

COPY --from=builder /go/bin/* /usr/local/bin/

ENTRYPOINT [ "peerswapd" ]