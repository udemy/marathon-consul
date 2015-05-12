FROM gliderlabs/alpine
MAINTAINER Brian Hicks <brian@brianthicks.com>

RUN apk add --update ca-certificates bash
COPY launch.sh /launch.sh

COPY . /go/src/github.com/CiscoCloud/marathon-consul
RUN apk add go git mercurial \
	&& cd /go/src/github.com/CiscoCloud/marathon-consul \
	&& export GOPATH=/go \
	&& go get \
	&& go build -o /bin/marathon-consul \
	&& rm -rf /go \
	&& apk del --purge go git mercurial

ENTRYPOINT ["/launch.sh"]
