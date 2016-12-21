FROM alpine:latest


RUN apk add --no-cache g++
ADD go_gymsearch /go_gymsearch
ADD ui /ui


CMD ["/go_gymsearch"]




