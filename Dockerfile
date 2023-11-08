# ---
FROM golang:latest

COPY . /src
WORKDIR /src

RUN apt-get update \
    && apt-get install -y ca-certificates
RUN CGO_ENABLED=0 go build -o ./gpe

# ---
FROM scratch

COPY --from=0 /etc/ssl/certs/ /etc/ssl/certs
COPY --from=0 /src/gpe /bin/gpe

EXPOSE 2112

ENTRYPOINT ["gpe"]