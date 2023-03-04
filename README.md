[![Go Report](https://goreportcard.com/badge/github.com/TopiSenpai/gobin)](https://goreportcard.com/report/github.com/TopiSenpai/gobin)
[![Go Version](https://img.shields.io/github/go-mod/go-version/TopiSenpai/gobin)](https://golang.org/doc/devel/release.html)
[![KittyBot License](https://img.shields.io/github/license/TopiSenpai/gobin)](LICENSE)
[![KittyBot Version](https://img.shields.io/github/v/tag/TopiSenpai/gobin?label=release)](https://github.com/TopiSenpai/gobin/releases/latest)
[![Docker](https://github.com/TopiSenpai/gobin/actions/workflows/docker.yml/badge.svg)](https://github.com/TopiSenpai/gobin/actions/workflows/docker.yml)
[![Discord](https://discordapp.com/api/guilds/608506410803658753/embed.png?style=shield)](https://discord.gg/sD3ABd5)

# gobin

gobin is a simple lightweight haste-server alternative written in Go, HTML, JS and CSS. It is aimed to be easy to use and deploy. You can find an instance running at [xgob.in](https://xgob.in).

<details>
<summary>Table of Contents</summary>

- [Features](#features)
- [Installation](#installation)
    - [Docker](#docker)
        - [Docker Compose](#docker-compose)
    - [Manual](#manual)
        - [Requirements](#requirements)
        - [Build](#build)
        - [Run](#run)
- [Configuration](#configuration)
- [Rate Limit](#rate-limits)
- [API](#api)
    - [Create a document](#create-a-document)
    - [Get a document](#get-a-document)
    - [Get a documents versions](#get-a-documents-versions)
    - [Get a document version](#get-a-document-version)
    - [Update a document](#update-a-document)
    - [Delete a document](#delete-a-document)
    - [Delete a document version](#delete-a-document-version)
    - [Other endpoints](#other-endpoints)
    - [Errors](#errors)
- [License](#license)
- [Contributing](#contributing)
- [Credits](#credits)
- [Contact](#contact)

</details>

## Features

- Easy to deploy and use
- Built-in rate-limiting
- Create, update and delete documents
- Syntax highlighting
- Document expiration
- Supports [PostgreSQL](https://www.postgresql.org/) or [SQLite](https://sqlite.org/)
- One binary and config file
- Docker image available
- ~~Metrics (to be implemented)~~

## Installation

### Docker

The easiest way to deploy gobin is using docker with [Docker Compose](https://docs.docker.com/compose/). You can find the docker image on [Packages](https://github.com/TopiSenpai/gobin/pkgs/container/gobin).

#### Docker Compose

Create a new `docker-compose.yml` file with the following content:

> **Note**
> You should change the password in the `docker-compose.yml` and `gobin.json` file.

```yaml
version: "3.8"

services:
  gobin:
    image: ghcr.io/topisenpai/gobin:latest
    container_name: gobin
    restart: unless-stopped
    volumes:
      - ./gobin.json:/var/lib/gobin/gobin.json
      # use this for sqlite
      - ./gobin.db:/var/lib/gobin/gobin.db
    ports:
      - 80:80

  # or use this for postgres
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

For `gobin.json`/environment variables and database schema see [Configuration](#configuration).

```bash
docker-compose up -d
```

---

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

---

## Configuration

The database schema is automatically created when you start gobin and there is no `documents` table in the database.

Create a new `gobin.json` file with the following content:

> **Note**
> Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

```yml
{
  "dev_mode": false,
  "debug": false,
  "listen_addr": "0.0.0.0:80",
  # secret for jwt tokens, replace with a long random string
  "jwt_secret": "...",
  "database": {
    # either "postgres" or "sqlite"
    "type": "postgres",
    "debug": false,
    "expire_after": "168h",
    "cleanup_interval": "10m",

    # path to sqlite database
    # if you run gobin with docker make sure to set it to "/var/lib/gobin/gobin.db"
    "path": "gobin.db",

    # postgres connection settings
    "host": "localhost",
    "port": 5432,
    "username": "gobin",
    "password": "password",
    "database": "gobin",
    "ssl_mode": "disable"
  },
  # max document size in characters
  "max_document_size": 0,
  # omit or set values to 0 or "0" to disable rate limit
  "rate_limit": {
    # number of requests which can be done in the duration
    "requests": 10,
    # the duration of the requests
    "duration": "1m"
  }
}
```

Alternatively you can use environment variables to configure gobin. The environment variables are prefixed with `GOBIN_` and are in uppercase. For example `GOBIN_DATABASE_TYPE` or `GOBIN_RATE_LIMIT_REQUESTS`.

<details>
<summary>Here is a list of all environment variables</summary>

```env
GOBIN_DEV_MODE=false
GOBIN_DEBUG=false
GOBIN_LISTEN_ADDR=0.0.0.0:80
GOBIN_JWT_SECRET=...

GOBIN_DATABASE_TYPE=postgres
GOBIN_DATABASE_DEBUG=false
GOBIN_DATABASE_EXPIRE_AFTER=168h
GOBIN_DATABASE_CLEANUP_INTERVAL=10m

GOBIN_DATABASE_PATH=gobin.db

GOBIN_DATABASE_HOST=localhost
GOBIN_DATABASE_PORT=5432
GOBIN_DATABASE_USERNAME=gobin
GOBIN_DATABASE_PASSWORD=password
GOBIN_DATABASE_DATABASE=gobin
GOBIN_DATABASE_SSL_MODE=disable

GOBIN_MAX_DOCUMENT_SIZE=0

GOBIN_RATE_LIMIT_REQUESTS=10
GOBIN_RATE_LIMIT_DURATION=1m
```

</details>

---

## Rate Limits

Following endpoints are rate-limited:

- `POST` `/documents`
- `PATCH` `/documents/{key}`
- `DELETE` `/documents/{key}`

`PATCH` and `DELETE` share the same bucket while `POST` has it's own bucket

---

## API

### Create a document

To create a paste you have to send a `POST` request to `/documents` with the `content` as `plain/text` body.

> **Note**
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
  "version": 1,
  "token": "kiczgez33j7qkvqdg9f7ksrd8jk88wba"
}
```

---

### Get a document

To get a document you have to send a `GET` request to `/documents/{key}`.

The response will be a `200 OK` with the document content as `application/json` body.

```json
{
  "key": "hocwr6i6",
  "version": "1",
  "data": "package main\n\nfunc main() {\n    println(\"Hello World!\")\n}",
  "language": "go"
}
```

---

### Get a documents versions

To get a documents versions you have to send a `GET` request to `/documents/{key}/versions?withData={bool}`.

The response will be a `200 OK` with the document content as `application/json` body.

```json
[
  {
    "version": 1,
    "data": "package main\n\nfunc main() {\n    println(\"Hello World!\")\n}",
    "language": "go"
  },
  {
    "version": 2,
    "data": "package main\n\nfunc main() {\n    println(\"Hello World2!\")\n}",
    "language": "go"
  }
]
```

### Get a document version

To get a document version you have to send a `GET` request to `/documents/{key}/versions/{version}`.

The response will be a `200 OK` with the document content as `application/json` body.

```json
{
  "key": "hocwr6i6",
  "version": 1,
  "data": "package main\n\nfunc main() {\n    println(\"Hello World!\")\n}",
  "language": "go"
}
```

---

### Update a document

To update a paste you have to send a `PATCH` request to `/documents/{key}` with the `content` as `plain/text` body and the `token` as `Authorization` header.

> **Note**
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

> **Note**
> The update token will not change after updating the document. You can use the same token to update the document again.

```json
{
  "key": "hocwr6i6",
  "version": 2
}
```

---

### Delete a document

To delete a document you have to send a `DELETE` request to `/documents/{key}` with the `token` as `Authorization` header.

A successful request will return a `204 No Content` response with an empty body.

---

### Delete a document version

To delete a document version you have to send a `DELETE` request to `/documents/{key}/versions/{version}` with the `token` as `Authorization` header.

A successful request will return a `204 No Content` response with an empty body.

---

### Other endpoints

- `GET` `/raw/{key}` - Get the raw content of a document
- `GET` `/raw/{key}/{version}` - Get the raw content of a document version
- `HEAD` `/raw/{key}` - Get the raw content of a document without the body
- `HEAD` `/raw/{key}/{version}` - Get the raw content of a document version without the body
- `GET` `/ping` - Get the status of the server
- `GET` `/debug` - Proof debug endpoint (only available in debug mode)
- `GET` `/version` - Get the version of the server

---

### Errors

In case of an error gobin will return the following JSON body with the corresponding HTTP status code:

```yaml
{
  "message": "document not found", # error message
  "status": 404, # HTTP status code
  "path": "/documents/7df3vw", # request path
  "request_id": "fbe0a365387f/gVAMGuraLW-003490" # request id
}
```

---

## License

gobin is licensed under the [Apache License 2.0](/LICENSE).

---

## Contributing

Contributions are always welcome! Just open a pull request or discussion and I will take a look at it.

---

## Credits

- [@Damon](https://github.com/day-mon) for helping me.

---

## Contact

- [Discord](https://discord.gg/sD3ABd5)
- [Twitter](https://twitter.com/TopiSenpai)
- [Email](mailto:git@topi.wtf)
