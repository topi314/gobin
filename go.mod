module github.com/topisenpai/gobin

go 1.20

replace (
	github.com/alecthomas/chroma/v2 => github.com/topisenpai/chroma/v2 v2.0.0-20230324233532-b14d693aac4a
	github.com/riandyrn/otelchi => github.com/TopiSenpai/otelchi v0.0.0-20230414141715-b42cdf8c6062
)

require (
	github.com/XSAM/otelsql v0.22.0
	github.com/alecthomas/chroma/v2 v2.7.0
	github.com/dustin/go-humanize v1.0.1
	github.com/go-chi/chi/v5 v5.0.8
	github.com/go-chi/httprate v0.7.4
	github.com/go-chi/stampede v0.5.1
	github.com/go-jose/go-jose/v3 v3.0.0
	github.com/jackc/pgx/v5 v5.3.1
	github.com/jmoiron/sqlx v1.3.5
	github.com/mitchellh/mapstructure v1.5.0
	github.com/prometheus/client_golang v1.15.0
	github.com/riandyrn/otelchi v0.5.1
	github.com/spf13/cobra v1.6.1
	github.com/spf13/viper v1.15.0
	go.opentelemetry.io/otel v1.15.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.15.0
	go.opentelemetry.io/otel/exporters/prometheus v0.38.0
	go.opentelemetry.io/otel/metric v0.38.0
	go.opentelemetry.io/otel/sdk v1.15.0
	go.opentelemetry.io/otel/sdk/metric v0.38.0
	go.opentelemetry.io/otel/trace v1.15.0
	golang.org/x/exp v0.0.0-20230425010034-47ecfdc1ba53
	modernc.org/sqlite v1.22.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dlclark/regexp2 v1.9.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.15.2 // indirect
	github.com/hashicorp/golang-lru v0.5.5-0.20200511160909-eb529947af53 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/lib/pq v1.10.7 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/pelletier/go-toml/v2 v2.0.7 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	go.opentelemetry.io/contrib v1.16.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.15.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.15.0 // indirect
	go.opentelemetry.io/proto/otlp v0.19.0 // indirect
	golang.org/x/crypto v0.8.0 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/tools v0.8.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.54.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/uint128 v1.3.0 // indirect
	modernc.org/cc/v3 v3.40.0 // indirect
	modernc.org/ccgo/v3 v3.16.13 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/opt v0.1.3 // indirect
	modernc.org/strutil v1.1.3 // indirect
	modernc.org/token v1.1.0 // indirect
)
