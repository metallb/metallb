#!/usr/bin/env bash

res=0

for i in `find . \( -path ./vendor -o -path ./vpp \) -prune -o -name "*.md"`
do
    if [ -d "$i" ]; then
        continue
    fi

    out=$(FORCE_COLOR=1 markdown-link-check -q "$i")
    if [ "$?" -ne 0 ]; then
    	echo "${out}"
        res=1
    fi
done

exit $res
