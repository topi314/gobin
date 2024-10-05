FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG COMMIT
ARG BUILD_TIME

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build -ldflags="-X 'main.Version=$VERSION' -X 'main.Commit=$COMMIT' -X 'main.BuildTime=$BUILD_TIME'" -o gobin-server github.com/topi314/gobin/v2

FROM alpine

RUN apk add --no-cache  \
    inkscape \
    ttf-freefont

COPY --from=build /build/gobin-server /bin/gobin

EXPOSE 80

ENTRYPOINT ["/bin/gobin"]

CMD ["-config", "/var/lib/gobin/gobin.toml"]
