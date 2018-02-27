# env GOOS=linux GOARCH=amd64 go build -o xsyn-deploy *.go

FROM centurylink/ca-certs
MAINTAINER Harry Denholm <harry.denholm@protonmail.com>

ENV XS_SRV_MESSAGE="xSyn, on Docker"
ENV XS_BOLT_FILE="/data/storage.db"
VOLUME /data
WORKDIR /app

COPY prod.toml /app/
COPY xsyn-deploy /app/

EXPOSE 8080
ENTRYPOINT ["./xsyn-deploy"]
