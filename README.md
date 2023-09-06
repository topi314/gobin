[![Go Report](https://goreportcard.com/badge/github.com/topi314/gobin)](https://goreportcard.com/report/github.com/topi314/gobin)
[![Go Version](https://img.shields.io/github/go-mod/go-version/topi314/gobin)](https://golang.org/doc/devel/release.html)
[![KittyBot License](https://img.shields.io/github/license/topi314/gobin)](LICENSE)
[![KittyBot Version](https://img.shields.io/github/v/tag/topi314/gobin?label=release)](https://github.com/topi314/gobin/releases/latest)
[![Docker](https://github.com/topi314/gobin/actions/workflows/docker.yml/badge.svg)](https://github.com/topi314/gobin/actions/workflows/docker.yml)
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
- Social Media PNG previews
- Document expiration
- Supports [PostgreSQL](https://www.postgresql.org/) or [SQLite](https://sqlite.org/)
- One binary and config file
- Docker image available
- ~~Metrics (to be implemented)~~

## Installation

### Docker

The easiest way to deploy gobin is using docker with [Docker Compose](https://docs.docker.com/compose/). You can find the docker image on [Packages](https://github.com/topi314/gobin/pkgs/container/gobin).

#### Docker Compose

Create a new `docker-compose.yml` file with the following content:

> **Note**
> You should change the password in the `docker-compose.yml` and `gobin.json` file.

```yaml
version: "3.8"

services:
  gobin:
    image: ghcr.io/topi314/gobin:latest
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

- Go 1.20 or higher
- PostgreSQL 13 or higher

#### Build

```bash
git clone https://github.com/topi314/gobin.git
cd gobin
go build -o gobin
```

or

```bash
go install github.com/topi314/gobin@latest
```

#### Run

```bash
gobin --config=gobin.json
```

---

## Configuration

The database schema is automatically created when you start gobin and there is no `documents` table in the database.

Create a new `gobin.json` file with the following content:

> **Note**
> Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

```yml
{
  "log": {
    # log level, either "debug", "info", "warn" or "error"
    "level": "info",
    # log format, either "json" or "text"
    "format": "text",
    # whether to add the source file and line to the log output
    "add_source": false
  },
  # enable or disable debug profiler endpoint
  "debug": false,
  # enable or disable hot reload of templates and assets
  "dev_mode": false,
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
    "duration": "1m",
    # a list of ip addresses which are exempt from rate limiting
    "whitelist": ["127.0.0.1"],
    # a list of ip addresses which are blocked from rate limited endpoints
    "blacklist": ["123.456.789.0"]
  },
  # settings for social media previews, omit to disable
  "previews": {
    # path to inkscape binary https://inkscape.org/
    "inkscape_path": "/usr/bin/inkscape",
    # how many lines should be shown in the preview
    "max_lines": 10,
    # how high the resolution of the preview should be, 96 is the default
    "dpi": 96,
    # how many previews should be maximally cached
    "cache_size": 1024,
    # how long should previews be cached
    "cache_duration": "1h"
  }
}
```

Alternatively you can use environment variables to configure gobin. The environment variables are prefixed with `GOBIN_` and are in uppercase. For example `GOBIN_DATABASE_TYPE` or `GOBIN_RATE_LIMIT_REQUESTS`.

<details>
<summary>Here is a list of all environment variables</summary>

```env
GOBIN_LOG_LEVEL=info
GOBIN_LOG_FORMAT=text
GOBIN_LOG_ADD_SOURCE=false

GOBIN_DEBUG=false
GOBIN_DEV_MODE=false
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

GOBIN_PREVIEW_INKSCAPE_PATH=/usr/bin/inkscape
GOBIN_PREVIEW_MAX_LINES=10
GOBIN_PREVIEW_DPI=96
GOBIN_PREVIEW_CACHE_SIZE=1024
GOBIN_PREVIEW_CACHE_TTL=1h
```

</details>

---

## Rate Limits

Following endpoints are rate-limited:

- `POST` `/documents`
- `PATCH` `/documents/{key}`
- `DELETE` `/documents/{key}`

`PATCH` and `DELETE` share the same bucket while `POST` has its own bucket

---

## API

Fields marked with `?` are optional and types marked with `?` are nullable.

### Formatter Enum

Document formatting is done using [chroma](https://github.com/alecthomas/chroma). The following formatters are available:

| Value           | Description             |
|-----------------|-------------------------|
| terminal8       | 8-bit terminal colors   |
| terminal16      | 16-bit terminal colors  |
| terminal256     | 256-bit terminal colors |
| terminal16m     | true terminal colors    |
| html            | HTML                    |
| html-standalone | Standalone HTML         |
| svg             | SVG                     |

---

### Language Enum

The following languages are available:

<details>
<summary>Click to expand</summary>

| Prefix | Language                                                                                                                                                                                                          |
|--------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| A      | ABAP, ABNF, ActionScript, ActionScript 3, Ada, Angular2, ANTLR, ApacheConf, APL, AppleScript, Arduino, Awk                                                                                                        |
| B      | Ballerina, Bash, Batchfile, BibTeX, Bicep, BlitzBasic, BNF, Brainfuck, BQN                                                                                                                                        |
| C      | C, C#, C++, Caddyfile, Caddyfile Directives, Cap'n Proto, Cassandra CQL, Ceylon, CFEngine3, cfstatement, ChaiScript, Chapel, Cheetah, Clojure, CMake, COBOL, CoffeeScript, Common Lisp, Coq, Crystal, CSS, Cython |
| D      | D, Dart, Diff, Django/Jinja, Docker, DTD, Dylan                                                                                                                                                                   |
| E      | EBNF, Elixir, Elm, EmacsLisp, Erlang                                                                                                                                                                              |
| F      | Factor, Fish, Forth, Fortran, FSharp                                                                                                                                                                              |
| G      | GAS, GDScript, Genshi, Genshi HTML, Genshi Text, Gherkin, GLSL, Gnuplot, Go, Go HTML Template, Go Text Template, GraphQL, Groff, Groovy                                                                           |
| H      | Handlebars, Haskell, Haxe, HCL, Hexdump, HLB, HLSL, HTML, HTTP, Hy                                                                                                                                                |
| I      | Idris, Igor, INI, Io                                                                                                                                                                                              |
| J      | J, Java, JavaScript, JSON, Julia, Jungle                                                                                                                                                                          |
| K      | Kotlin                                                                                                                                                                                                            |
| L      | Lighttpd configuration file, LLVM, Lua                                                                                                                                                                            |
| M      | Makefile, Mako, markdown, Mason, Mathematica, Matlab, MiniZinc, MLIR, Modula-2, MonkeyC, MorrowindScript, Myghty, MySQL                                                                                           |
| N      | NASM, Newspeak, Nginx configuration file, Nim, Nix                                                                                                                                                                |
| O      | Objective-C, OCaml, Octave, OnesEnterprise, OpenEdge ABL, OpenSCAD, Org Mode                                                                                                                                      |
| P      | PacmanConf, Perl, PHP, PHTML, Pig, PkgConfig, PL/pgSQL, plaintext, Pony, PostgreSQL SQL dialect, PostScript, POVRay, PowerShell, Prolog, PromQL, Properties, Protocol Buffer, PSL, Puppet, Python 2, Python       |
| Q      | QBasic                                                                                                                                                                                                            |
| R      | R, Racket, Ragel, Raku, react, ReasonML, reg, reStructuredText, Rexx, Ruby, Rust                                                                                                                                  |
| S      | SAS, Sass, Scala, Scheme, Scilab, SCSS, Sed, Smalltalk, Smarty, Snobol, Solidity, SPARQL, SQL, SquidConf, Standard ML, stas, Stylus, Svelte, Swift, SYSTEMD, systemverilog                                        |
| T      | TableGen, TASM, Tcl, Tcsh, Termcap, Terminfo, Terraform, TeX, Thrift, TOML, TradingView, Transact-SQL, Turing, Turtle, Twig, TypeScript, TypoScript, TypoScriptCssData, TypoScriptHtmlData                        |
| V      | VB.net, verilog, VHDL, VHS, VimL, vue                                                                                                                                                                             |
| W      | WDTE                                                                                                                                                                                                              |
| X      | XML, Xorg                                                                                                                                                                                                         |
| Y      | YAML, YANG                                                                                                                                                                                                        |
| Z      | Zig                                                                                                                                                                                                               |

</details>

--- 

### Create a document

To create a paste you have to send a `POST` request to `/documents` with the `content` as `plain/text` body.

| Query Parameter | Type                         | Description                                  |
|-----------------|------------------------------|----------------------------------------------|
| language?       | [language](#language-enum)   | The language of the document.                |
| formatter?      | [formatter](#formatter-enum) | With which formatter to render the document. |

```go
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

| Query Parameter | Type                         | Description                                        |
|-----------------|------------------------------|----------------------------------------------------|
| language?       | [language](#language-enum)   | In which language the document should be rendered. |
| formatter?      | [formatter](#formatter-enum) | With which formatter to render the document        |

The response will be a `200 OK` with the document content as `application/json` body.

```yaml
{
  "key": "hocwr6i6",
  "version": "1",
  "data": "package main\n\nfunc main() {\n    println(\"Hello World!\")\n}",
  "formatted": "...", # only if formatter is set
  "css": "...", # only if formatter=html
  "language": "go"
}
```

---

### Get a documents versions

To get a documents versions you have to send a `GET` request to `/documents/{key}/versions`.

| Query Parameter | Type                         | Description                                        |
|-----------------|------------------------------|----------------------------------------------------|
| withData?       | bool                         | If the data should be included in the response.    |
| language?       | [language](#language-enum)   | In which language the document should be rendered. |
| formatter?      | [formatter](#formatter-enum) | The formatter to use for rendering the document.   |

The response will be a `200 OK` with the document content as `application/json` body.

```yaml
[
  {
    "version": 1,
    "data": "package main\n\nfunc main() {\n    println(\"Hello World!\")\n}",
    "formatted": "...", # only if formatter is set
    "css": "...", # only if formatter=html
    "language": "go"
  },
  {
    "version": 2,
    "data": "package main\n\nfunc main() {\n    println(\"Hello World2!\")\n}",
    "formatted": "...", # only if formatter is set
    "css": "...", # only if formatter=html
    "language": "go"
  }
]
```

### Get a document version

To get a document version you have to send a `GET` request to `/documents/{key}/versions/{version}`.

| Query Parameter | Type                         | Description                                        |
|-----------------|------------------------------|----------------------------------------------------|
| language?       | [language](#language-enum)   | In which language the document should be rendered. |
| formatter?      | [formatter](#formatter-enum) | With which formatter to render the document.       |

The response will be a `200 OK` with the document content as `application/json` body.

```yaml
{
  "key": "hocwr6i6",
  "version": 1,
  "data": "package main\n\nfunc main() {\n    println(\"Hello World!\")\n}",
  "formatted": "...", # only if formatter is set
  "css": "...", # only if formatter=html
  "language": "go"
}
```

---

### Update a document

To update a paste you have to send a `PATCH` request to `/documents/{key}` with the `content` as `plain/text` body and the `token` as `Authorization` header.

| Query Parameter | Type                         | Description                                  |
|-----------------|------------------------------|----------------------------------------------|
| language?       | [language](#language-enum)   | The language of the document.                |
| formatter?      | [formatter](#formatter-enum) | With which formatter to render the document. |

```
Authorization: kiczgez33j7qkvqdg9f7ksrd8jk88wba
```

```go
package main

func main() {
    println("Hello World Updated!")
}
```

A successful request will return a `200 OK` response with a JSON body containing the document key and token to update the document.

> **Note**
> The update token will not change after updating the document. You can use the same token to update the document again.

```yaml
{
  "key": "hocwr6i6",
  "version": 2,
  "data": "package main\n\nfunc main() {\n    println(\"Hello World Updated!\")\n}", # only if formatter is set
  "formatted": "...", # only if formatter is set
  "css": "...", # only if formatter=html
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

- `GET`/`HEAD` `/{key}/preview` - Get the preview of a document, query parameters are the same as for `GET /documents/{key}`
- `GET`/`HEAD` `/{key}/{version}/preview` - Get the preview of a document version, query parameters are the same as for `GET /documents/{key}/versions/{version}`
- `GET`/`HEAD` `/documents/{key}/preview` - Get the preview of a document, query parameters are the same as for `GET /documents/{key}`
- `GET`/`HEAD` `/documents/{key}/versions/{version}/preview` - Get the preview of a document version, query parameters are the same as for `GET /documents/{key}/versions/{version}`
- `GET`/`HEAD` `/raw/{key}` - Get the raw content of a document, query parameters are the same as for `GET /documents/{key}`
- `GET`/`HEAD` `/raw/{key}/{version}` - Get the raw content of a document version, query parameters are the same as for `GET /documents/{key}/versions/{version}`
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
- [Twitter](https://twitter.com/topi314)
- [Email](mailto:git@topi.wtf)
