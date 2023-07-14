FROM alpine
MAINTAINER label <wby@ojbk.io>

WORKDIR /gophercron
COPY ./_build/config /gophercron/config
COPY ./_build/view /gophercron/view
COPY ./_build/gophercron /gophercron/gophercron
CMD ["./gophercron", "service", "-c", "./config/service-config-default.toml"]