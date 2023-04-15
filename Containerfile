FROM docker.io/library/golang:1.19.6 AS builder

COPY . /src
WORKDIR /src
RUN go build -v .

FROM registry.fedoraproject.org/fedora:37

RUN curl -Lo - https://github.com/juruen/rmapi/releases/download/v0.0.25/rmapi-linuxx86-64.tar.gz \
  | tar -xzf - -C /bin

COPY --from=builder /src/bookmaker /bin/

CMD ["bookmaker"]
