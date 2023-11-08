# ---
FROM golang:1.21

ARG GPE_VERSION "dev"

COPY . /src
WORKDIR /src

RUN apt-get update \
    && apt-get install -y ca-certificates
RUN CGO_ENABLED=0 go build -o ./gpe -ldflags="-X main.version=${GPE_VERSION}"

# ---
FROM scratch

COPY --from=0 /etc/ssl/certs/ /etc/ssl/certs
COPY --from=0 /src/gpe /bin/gpe

EXPOSE 2112

ENTRYPOINT ["gpe"]