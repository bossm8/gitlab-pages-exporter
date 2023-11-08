FROM golang:latest

RUN apt-get update && \
    apt-get install -y \
        git \
        vim \
        bash-completion

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
        | sh -s -- -b $(go env GOPATH)/bin v1.53.3
