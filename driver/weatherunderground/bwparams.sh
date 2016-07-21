#!/bin/bash

set -u

if [ -z ${1+x} ]; then
    echo "Usage: ./bwperms.sh from to deployNS"
    exit 1
fi

fromEntity=$1
toEntity=$2
deployNS=$3

echo "From $fromEntity"
echo "To $toEntity"
echo "Deploy On: $deployNS"

uri=$deployNS/s.weatherunderground/*
echo "Checking PC* to" $uri
bw2 bc -t $toEntity -u $uri -x 'PC*'
if [ $? != 0 ]; then
    echo "Granting PC* to" $uri
    bw2 mkdot -m "WeatherUnderground driver publish/subscribe" -e 5y -f $fromEntity -t $toEntity -u $uri -x 'PC*'
fi
