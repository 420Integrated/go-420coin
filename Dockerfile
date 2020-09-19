# Build G420 in a stock Go builder container
FROM golang:1.15-alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git

ADD . /go-420coin
RUN cd /go-420coin && make g420

# Pull G420 into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /go-420coin/build/bin/g420 /usr/local/bin/

EXPOSE 8545 8546 8547 13013 13013/udp
ENTRYPOINT ["g420"]
