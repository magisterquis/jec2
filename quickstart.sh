#!/bin/sh
#
# quickstart.sh
# Quickly set up a JEC2 server
# By J. Stuart McMurray
# Created 20220409
# Last Modified 20220409

set -e

# Make sure we have a server address
if [ "" == "$1" ] || [ "-h" == "$1" ]; then
        echo "Usage: $0 serveraddress" >&2
        exit 1
fi
SADDR="$1"


# wait_for_file waits for the file $1 to exist, which it calls $2.
function wait_for_file {
        echo -n "Waiting for $2... "
        for i in 1 2 3 4 5 6 7 8 9; do
                if [ -f "$1" ]; then
                        break
                fi
                sleep 1
                echo -n "$i... "
        done
        ls "$1"
}

# Build and start server
SVR="jeserver"
echo -n "Building server... "
go build -trimpath -o "$SVR" ./cmd/jeserver
ls "$SVR"
echo -n "Making working directory... "
DIR="$(./$SVR -print-dir)"
mkdir -p "$DIR"
ls -d "$DIR"
echo -n "Putting server in binary directory... "
BIN="$DIR/bin"
mkdir -p "$BIN"
mv "$SVR" "$BIN"
ls "$BIN/$SVR"
echo -n "Starting server... "
PATH="$BIN:$PATH" nohup jeserver >>$DIR/log 2>&1 &
SPID="$!"
sleep 1
if ! ps -p "$SPID" >/dev/null ; then
        echo "died in the first second, check $DIR/log for why"
        exit 1
else
        echo PID "$SPID"
fi

# Wait for key generation
LOGF="$DIR/log"
wait_for_file "$LOGF" "logfile creation"
SKEY="$DIR/id_ed25519_server"
wait_for_file "$SKEY" "server key generation"
IKEY="$DIR/id_ed25519_implant"
wait_for_file "$IKEY" "implant key generation"
OKEY="$DIR/id_ed25519_operator"
wait_for_file "$OKEY" "operator key generation"

# Make implant-builder
echo -n "Making implant directory... "
IDIR="$DIR/implants"
mkdir -p "$IDIR"
ls -d "$IDIR"
echo -n "Getting server fingerprint... "
FP="$(ssh-keygen -lf $SKEY | cut -f 2 -d ' ')"
echo "$FP"
BS="$BIN/jegenimplant.sh"
echo -n "Generating implant build script... "
./cmd/ibgen.sh "$SADDR" "$FP" "$IKEY" "$IDIR" > "$BS"
chmod 0700 "$BS"
ls "$BS"

# Build starter implants
for os in openbsd linux darwin windows; do
        echo -n "Building implant for $os... "
        GOOS="$os" GOARCH=amd64 "$BS"
done
