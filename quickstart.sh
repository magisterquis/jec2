#!/bin/sh
#
# quickstart.sh
# Quickly get a server up and running
# By J. Stuart McMurray
# Created 20220402
# Last Modified 20220402

set -e

# Make sure we have tools
for i in go jq; do 
        echo -n "Checking for command $i..."
        if ! which $i; then echo
                echo "Please ensure $i is in your PATH and try again" >&2
                exit 2
        fi
done

# Build the server
if ! [ -d ./cmd/jeserver ]; then
        echo "Directory cmd/jeserver not found" >&2
        echo "Please run this script from the root of the jec2 source tree"
        exit 1
fi
echo -n "Building jeserver..."
go install -trimpath ./cmd/jeserver
which jeserver

# Start server going with a default config
WD="${WD:-"$(jeserver -print-dir)"}"
echo -n "Starting JEServer in $WD..."
CMD="jeserver -work-dir "$WD" -log log"
nohup $CMD >/dev/null 2>&1 &
PID="$!"
echo "pid $PID"
echo -n "Waiting 5s to make sure JEServer stays alive..."
sleep 5
if ps -p $PID >/dev/null 2>&1; then
        echo "it did"
else
        echo "it did not; check $WD/log for more details"
        echo "The command used to start JEServer was:"
        echo "$CMD"
fi

# Make an implant
if [ -x ./buildimplant.sh ]; then
        ./buildimplant.sh
fi
