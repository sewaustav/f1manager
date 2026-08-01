#!/bin/sh
set -e

# Generate the RSA keypair the JWT middleware needs if it isn't already present.
# In compose these paths live on a named volume, so keys persist across restarts.
KEY_DIR="${KEY_DIR:-/keys}"
: "${JWT_PRIVATE_KEY_PATH:=$KEY_DIR/jwt_private.pem}"
: "${JWT_PUBLIC_KEY_PATH:=$KEY_DIR/jwt_public.pem}"
export JWT_PRIVATE_KEY_PATH JWT_PUBLIC_KEY_PATH

mkdir -p "$KEY_DIR"
if [ ! -f "$JWT_PRIVATE_KEY_PATH" ] || [ ! -f "$JWT_PUBLIC_KEY_PATH" ]; then
  echo "entrypoint: generating JWT RSA keys in $KEY_DIR"
  openssl genrsa -out "$JWT_PRIVATE_KEY_PATH" 2048
  openssl rsa -in "$JWT_PRIVATE_KEY_PATH" -pubout -out "$JWT_PUBLIC_KEY_PATH"
fi

exec "$@"
