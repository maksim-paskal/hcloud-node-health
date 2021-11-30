FROM alpine:latest

COPY ./hcloud-node-health /app/hcloud-node-health

WORKDIR /app

RUN addgroup -g 101 -S app \
&& adduser -u 101 -D -S -G app app

USER 101

ENTRYPOINT ["/app/hcloud-node-health"]