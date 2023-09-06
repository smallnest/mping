# mping
![GitHub](https://img.shields.io/github/license/smallnest/mping) ![GitHub Action](https://github.com/smallnest/mping/actions/workflows/go.yaml/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/smallnest/mping)](https://goreportcard.com/report/github.com/smallnest/mping)  [![GoDoc](https://godoc.org/github.com/smallnest/mping?status.png)](http://godoc.org/github.com/smallnest/mping)  

a multi-targets ping tool, which supports 10,000 packets/second

> 一个高频ping工具，支持多个目标。
> 正常的ping一般用来做探测工具，mping还可以用来做压测工具。

## Usage

compile

```sh
go build .
```

options usage.

```sh
> $$ mping -h     

Usage: mping [options] [host list]

  -c int
        count, 0 means non-setting
  -d int
        delay seconds (default 3)
  -l int
        packet size (default 64)
  -r int
        rate, 100 means 100 packets per second (default 100)
  -t duration
        timeout (default 1s)
  -z int
        tos, 0 means non-setting
  --bitflip bool
        check whether there is bit flipped
```

example

```sh
sudo ./mping -r 5 8.8.8.8
sudo ./mping -r 100 8.8.8.8,8.8.4.4
sudo ./mping -r 100 github.com,bing.com
```

docker:
```
sudo docker run --rm smallnest/mping -it mping 8.8.8.8
```