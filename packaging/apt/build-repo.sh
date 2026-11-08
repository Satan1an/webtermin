#!/usr/bin/env bash
# Build a signed APT repository from a directory of webtermin .deb files.
#
#   packaging/apt/build-repo.sh <debs-dir> <out-dir> <gpg-key-id>
#
# Layout produced in <out-dir> (served as-is by GitHub Pages):
#   pool/main/w/webtermin/*.deb
#   dists/stable/main/binary-{amd64,arm64}/Packages{,.gz}
#   dists/stable/{Release,InRelease,Release.gpg}
#   webtermin.gpg   public key, binary (dearmored) — for /etc/apt/keyrings
#   webtermin.asc   same key, ASCII-armored
#
# Needs apt-ftparchive (apt-utils), gpg and gzip. The secret key must already
# be in the default keyring; set GPG_PASSPHRASE if it is protected.
set -euo pipefail

debs=${1:?usage: build-repo.sh <debs-dir> <out-dir> <gpg-key-id>}
out=${2:?usage: build-repo.sh <debs-dir> <out-dir> <gpg-key-id>}
key=${3:?usage: build-repo.sh <debs-dir> <out-dir> <gpg-key-id>}

suite=stable
component=main
archs=(amd64 arm64)

shopt -s nullglob
files=("$debs"/*.deb)
if [ ${#files[@]} -eq 0 ]; then
  echo "no .deb files in $debs" >&2
  exit 1
fi

rm -rf "$out"
pool="pool/$component/w/webtermin"
mkdir -p "$out/$pool"
cp "${files[@]}" "$out/$pool/"

cd "$out"
for arch in "${archs[@]}"; do
  dir="dists/$suite/$component/binary-$arch"
  mkdir -p "$dir"
  # Paths in Packages are relative to the repo root, so run from there.
  apt-ftparchive --arch "$arch" packages pool > "$dir/Packages"
  gzip -9nk "$dir/Packages"
done

# Release must not list itself: generate into a temp file, then move.
tmp=$(mktemp)
apt-ftparchive \
  -o "APT::FTPArchive::Release::Origin=webtermin" \
  -o "APT::FTPArchive::Release::Label=webtermin" \
  -o "APT::FTPArchive::Release::Suite=$suite" \
  -o "APT::FTPArchive::Release::Codename=$suite" \
  -o "APT::FTPArchive::Release::Architectures=${archs[*]}" \
  -o "APT::FTPArchive::Release::Components=$component" \
  -o "APT::FTPArchive::Release::Description=webtermin server control panel" \
  release "dists/$suite" > "$tmp"
mv "$tmp" "dists/$suite/Release"
chmod 644 "dists/$suite/Release"

sign=(gpg --batch --yes --local-user "$key" --digest-algo SHA512)
if [ -n "${GPG_PASSPHRASE:-}" ]; then
  sign+=(--pinentry-mode loopback --passphrase-fd 0)
fi
printf '%s' "${GPG_PASSPHRASE:-}" | "${sign[@]}" --clearsign -o "dists/$suite/InRelease" "dists/$suite/Release"
printf '%s' "${GPG_PASSPHRASE:-}" | "${sign[@]}" --armor --detach-sign -o "dists/$suite/Release.gpg" "dists/$suite/Release"

gpg --batch --yes --export "$key" > webtermin.gpg
gpg --batch --yes --armor --export "$key" > webtermin.asc

echo "APT repo built in $out:"
find . -type f | sort
