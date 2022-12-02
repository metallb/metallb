#!/bin/bash
set -e

EXPECTED_DAEMONS=" bfdd bgpd staticd watchfrr zebra "
DAEMONS=$(vtysh -c "show daemons" | tr " " "\n" | sort | tr "\n" " ")

if [ "$DAEMONS" != "$EXPECTED_DAEMONS" ]; then
        echo "Did not find all the expected daemons [$DAEMONS]"
        exit 1
fi


