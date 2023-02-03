FROM golang:1.19-alpine AS build

WORKDIR /build

COPY tools/go.mod tools/go.sum tools/

RUN cd tools && go mod download

COPY tools/ tools/

RUN go run .

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o gobin .

FROM alpine

COPY --from=build /build/gobin /bin/gobin

EXPOSE 80

ENTRYPOINT ["/bin/gobin"]

CMD ["-config", "/var/lib/gobin/config.json"]
