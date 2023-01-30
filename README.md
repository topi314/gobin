[![Go Report](https://goreportcard.com/badge/github.com/TopiSenpai/gobin)](https://goreportcard.com/report/github.com/TopiSenpai/gobin)
[![Go Version](https://img.shields.io/github/go-mod/go-version/TopiSenpai/gobin)](https://golang.org/doc/devel/release.html)
[![KittyBot License](https://img.shields.io/github/license/TopiSenpai/gobin)](LICENSE)
[![KittyBot Version](https://img.shields.io/github/v/tag/TopiSenpai/gobin?label=release)](https://github.com/TopiSenpai/gobin/releases/latest)
[![Docker](https://github.com/TopiSenpai/gobin/actions/workflows/docker.yml/badge.svg)](https://github.com/TopiSenpai/gobin/actions/workflows/docker.yml)
[![Discord](https://discordapp.com/api/guilds/608506410803658753/embed.png?style=shield)](https://discord.gg/sD3ABd5)

# gobin

gobin is a simple lightweight haste-server alternative written in Go, HTML, JS and CSS. It is easy to deploy and use. You can find a public version at [xgob.in](https://xgob.in).

## Features

- Easy to deploy and use
- Create, update and delete documents
- Syntax highlighting
- Document expiration
- Only [PostgreSQL](https://www.postgresql.org/) required
- One binary and config file
- Docker image available

## Installation

### Docker

The easiest way to deploy gobin is using docker with [Docker Compose](https://docs.docker.com/compose/). You can find the docker image on [Packages](https://github.com/TopiSenpai/gobin/pkgs/container/gobin).

#### Docker Compose

Create a new `docker-compose.yml` file with the following content:

> **Note:**
> You should change the password in the `docker-compose.yml` and `config.json` file.

```yaml
version: "3.8"

services:
  gobin:
    image: ghcr.io/topisenpai/gobin:latest
    container_name: gobin
    restart: unless-stopped
    volumes:
      - ./config.json:/var/lib/gobin/config.json
    ports:
      - 80:80

  postgres:
    image: postgres:latest
    container_name: postgres
    restart: unless-stopped
    volumes:
      - ./data:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: gobin
      POSTGRES_USER: gobin
      POSTGRES_PASSWORD: password
```

For `config.json` and database see schema [Configuration](#configuration).

```bash
docker-compose up -d
```

### Manual

#### Requirements

- Go 1.18 or higher
- PostgreSQL 13 or higher

#### Build

```bash
git clone https://github.com/TopiSenpai/gobin.git
cd gobin
go build -o gobin
```

or

```bash
go install github.com/TopiSenpai/gobin@latest
```

#### Run

```bash
gobin --config=config.json
```

#### Configuration

Create a new table in your PostgreSQL database with the following schema:

```sql
CREATE TABLE documents
(
    id           VARCHAR PRIMARY KEY,
    content      TEXT      NOT NULL,
    language     VARCHAR   NOT NULL,
    update_token VARCHAR   NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL
);
```

Then create a new `config.json` file with the following content:

> **Note:**
> Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

```json
{
  "listen_addr": "0.0.0.0:80",
  "expire_after": "168h",
  "cleanup_interval": "10m",
  "database": {
    "host": "localhost",
    "port": 5432,
    "username": "gobin",
    "password": "password",
    "database": "gobin",
    "ssl_mode": "disable"
  }
}
```

## Usage

### Create a document

To create a paste you have to send a `POST` request to `/documents` with the `content` as `plain/text` body.

> **Note:**
> You can also specify the code language with the `Language` header.

```
Language: go

package main

func main() {
    println("Hello World!")
}
```

A successful request will return a `200 OK` response with a JSON body containing the document key and token to update the document.

```json
{
  "key": "hocwr6i6",
  "update_token": "kiczgez33j7qkvqdg9f7ksrd8jk88wba"
}
```

### Update a document

To update a paste you have to send a `PATCH` request to `/documents/{key}` with the `content` as `plain/text` body and the `update_token` as `Authorization` header.

> **Note:**
> You can also specify the code language with the `Language` header.

```
Authorization: kiczgez33j7qkvqdg9f7ksrd8jk88wba
Language: go

package main

func main() {
    println("Hello World Updated!")
}
```

A successful request will return a `200 OK` response with a JSON body containing the document key and token to update the document.

> **Note:**
> The update token will not change after updating the document. You can use the same token to update the document again.

```json
{
  "key": "hocwr6i6",
  "update_token": "kiczgez33j7qkvqdg9f7ksrd8jk88wba"
}
```

### Delete a document

To delete a paste you have to send a `DELETE` request to `/documents/{key}` with the `update_token` as `Authorization` header.

## License

gobin is licensed under the [Apache License 2.0](/LICENSE).

## Contributing

Contributions are always welcome! Just open a pull request or discussion and I will take a look at it.

## Credits

- [@Damon](https://github.com/day-mon) for helping me.

## Contact

- [Discord](https://discord.gg/sD3ABd5)
- [Twitter](https://twitter.com/TopiSenpai)
- [Email](mailto:git@topi.wtf)
