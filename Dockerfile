FROM amd64/golang:1.22.0-alpine3.18 AS builder

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /gophercron
COPY . .
RUN mkdir -p _build
COPY dist /gophercron/_build/view
COPY ./cmd/service/conf/config-default.toml /gophercron/_build/config/service-config-default.toml
COPY ./cmd/client/conf/config-default.toml /gophercron/_build/config/client-config-default.toml
RUN go build -a -ldflags '-extldflags "-static"' -o _build/gophercron ./cmd/


FROM amd64/alpine:3.18
LABEL MAINTAINER <w@ojbk.io>

RUN apk update && apk add tzdata diffutils curl && cp -r -f /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

WORKDIR /gophercron
COPY --from=builder /gophercron/_build/config /gophercron/config
COPY --from=builder /gophercron/_build/view /gophercron/view
COPY --from=builder /gophercron/_build/gophercron /gophercron/gophercron

CMD ["./gophercron", "service", "-c", "./config/service-config-default.toml"]