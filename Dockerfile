FROM alpine:latest

ADD ui/ /ui/ 
ADD gymsearch /gymsearch
RUN apk add --update ca-certificates && \
    rm -rf /var/cache/apk/* /tmp/*
RUN apk add -U tzdata
RUN update-ca-certificates
EXPOSE 9000
CMD ["/gymsearch"]




