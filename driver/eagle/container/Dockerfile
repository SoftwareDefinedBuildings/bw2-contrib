FROM ubuntu:xenial
MAINTAINER Gabe Fierro <gtfierro@eecs.berkeley.edu>

RUN apt-get -y update && apt-get install -y git libssl-dev
RUN apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ADD eagle /bin/eagle

ENTRYPOINT [ "/bin/eagle" ]
