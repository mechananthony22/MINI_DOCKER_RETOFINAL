#!/bin/bash
set -e

# Asegurar que se corre con sudo si va a escribir a /tmp/rootfs
if [ "$EUID" -ne 0 ]; then
  echo "Por favor corre este script como root (sudo)"
  exit
fi

ROOTFS_DIR="/tmp/rootfs"

echo "Creando directorio en $ROOTFS_DIR..."
mkdir -p "$ROOTFS_DIR"

echo "Descargando Alpine Linux mini root filesystem..."
# Usaremos curl para descargar el tarball. 
# Si el link se cae, actualizar al alpine minirootfs más reciente desde su web oficial
ALPINE_URL="https://dl-cdn.alpinelinux.org/alpine/v3.18/releases/x86_64/alpine-minirootfs-3.18.4-x86_64.tar.gz"

cd "$ROOTFS_DIR"
curl -O "$ALPINE_URL"

echo "Extrayendo el sistema de archivos..."
tar -xf alpine-minirootfs-3.18.4-x86_64.tar.gz
rm alpine-minirootfs-3.18.4-x86_64.tar.gz

echo "Rootfs preparado en $ROOTFS_DIR"

echo "Configurando resolución DNS (resolv.conf)..."
# Usamos -L (o cp --dereference) para asegurar que copiamos el archivo real 
# y no un enlace simbólico que se rompería dentro del contenedor
cp -L /etc/resolv.conf "$ROOTFS_DIR/etc/resolv.conf"
