############################
# STEP 1 build executable binary
############################
FROM golang:1.21 as builder

workdir /work

# If you encounter some issues when pulling modules, \
# you can try to use GOPROXY, especially in China.
ENV GOPROXY=https://goproxy.cn

COPY . /work

RUN CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -tags timetzdata -o mping .

############################
# STEP 2 build a small image
############################
FROM scratch
# Copy our static executable.
COPY --from=builder /work/mping /mping
# Run the mping binary.
ENTRYPOINT ["/mping"]

# Usage:
#
# build mping image
# docker build -t mping .
#
# print help
# docker run --rm -it --name mping -h
# docker run --rm -it --name mping 8.8.8.8
