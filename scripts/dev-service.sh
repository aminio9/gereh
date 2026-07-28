#!/usr/bin/env bash
set -Eeuo pipefail

service_name="${1:-}"

if [[ -z "${service_name}" ]]; then
  echo "Usage: $0 <service-name>" >&2
  exit 2
fi

service_dir="services/${service_name}"
main_package="./${service_dir}/cmd/${service_name}"

if [[ ! -d "${service_dir}" ]]; then
  echo "Unknown service: ${service_name}" >&2
  exit 2
fi

mkdir -p .tmp

config_file=".tmp/air-${service_name}.toml"
cat >"${config_file}" <<EOF
root = "."
tmp_dir = ".tmp/${service_name}"

[build]
cmd = "go build -o .tmp/${service_name}/${service_name} ${main_package}"
bin = ".tmp/${service_name}/${service_name}"
include_ext = ["go", "sql", "proto"]
exclude_dir = [".git", ".tmp", "apps", "gen", "node_modules"]
delay = 300
stop_on_error = true
send_interrupt = true
kill_delay = "3s"

[log]
time = true

[misc]
clean_on_exit = true
EOF

exec air -c "${config_file}"
