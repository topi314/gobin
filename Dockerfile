FROM golang:1.19-alpine AS build

ARG GITHUB_TOKEN
ARG VERSION
ARG COMMIT
ARG BUILD_TIME

WORKDIR /build

COPY tools/go.mod tools/go.sum tools/

RUN cd tools && go mod download

COPY tools/ tools/

RUN mkdir assets && \
    mkdir assets/styles && \
    cd tools && \
    go run . --github-token $GITHUB_TOKEN

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-X 'main.version=$VERSION' -X 'main.commit=$COMMIT' -X 'main.buildTime=$BUILD_TIME'" -o gobin-server github.com/topisenpai/gobin

FROM alpine

COPY --from=build /build/gobin-server /bin/gobin

EXPOSE 80

ENTRYPOINT ["/bin/gobin"]

CMD ["-config", "/var/lib/gobin/config.json"]
