# mping
a multi-targets ping tool, which supports 10,000 packets/second

> 一个高频ping工具，支持多个目标。
> 正常的ping一般用来做探测工具，mping还可以用来做压测工具。

## 使用

```
go build .
sudo ./mping -r 5 8.8.8.8
sudo ./mping -r 100 8.8.8.8,8.8.4.4
```