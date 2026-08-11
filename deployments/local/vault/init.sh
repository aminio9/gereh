#!/bin/sh

set -eu

export VAULT_ADDR="${VAULT_ADDR:-http://vault:8200}"

until vault status >/dev/null 2>&1; do
  sleep 1
done

if ! vault secrets list -format=json |
  grep -q '"model-byok/"'; then

  vault secrets enable \
    -path=model-byok \
    -version=2 \
    kv
fi

vault policy write \
  gereh-model-access \
  - <<'EOF'
path "model-byok/data/tenants/*" {
  capabilities = ["create", "update", "read"]
}

path "model-byok/metadata/tenants/*" {
  capabilities = ["read", "delete"]
}

path "model-byok/destroy/tenants/*" {
  capabilities = ["update"]
}
EOF

mkdir -p /var/run/gereh/vault

vault token create \
  -orphan \
  -policy=gereh-model-access \
  -ttl=24h \
  -field=token \
  > /var/run/gereh/vault/model-access.token

dd if=/dev/urandom \
  bs=32 \
  count=1 \
  2>/dev/null |
  base64 \
  > /var/run/gereh/vault/model-access-fingerprint.key

chown 1000:1000 \
  /var/run/gereh/vault/model-access.token \
  /var/run/gereh/vault/model-access-fingerprint.key

chmod 0400 \
  /var/run/gereh/vault/model-access.token \
  /var/run/gereh/vault/model-access-fingerprint.key

echo "Vault BYOK development configuration initialized."
