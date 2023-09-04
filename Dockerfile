FROM golang:1.21 as build

workdir /work

# If you encounter some issues when pulling modules, \
# you can try to use GOPROXY, especially in China.
# ENV GOPROXY=https://goproxy.cn

COPY . /work

RUN go build -o mping .

ENTRYPOINT ["/work/mping"]

# Usage:
#
# build mping image
# docker build -t mping .
#
# print help
# docker run --rm --name mping -it mping -h
# docker run --rm --name mping -it mping 8.8.8.8
