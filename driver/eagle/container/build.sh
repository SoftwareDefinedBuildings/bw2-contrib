#!/bin/bash
set -ex
cd .. ; go build ; cd - ; cp ../eagle .
docker build -t gtfierro/eagle .
docker push gtfierro/eagle
