FROM golang:1.20-alpine AS build

ARG VERSION
ARG COMMIT
ARG BUILD_TIME

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-X 'main.version=$VERSION' -X 'main.commit=$COMMIT' -X 'main.buildTime=$BUILD_TIME'" -o gobin-server github.com/topisenpai/gobin

FROM alpine

COPY --from=build /build/gobin-server /bin/gobin

EXPOSE 80

ENTRYPOINT ["/bin/gobin"]

CMD ["-config", "/var/lib/gobin/config.json"]
