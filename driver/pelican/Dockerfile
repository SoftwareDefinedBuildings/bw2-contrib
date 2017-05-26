FROM jhkolb/spawnd:amd64
MAINTAINER Jack Kolb <jkolb@berkeley.edu>

RUN apt-get update && apt-get install -y ca-certificates
add pelican /usr/bin/pelican
CMD ["/usr/local/bin/pelican"]
