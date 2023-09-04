# mping
a multi-targets ping tool, which supports 10,000 packets/second

[status] developing

## 使用

```
go build .
sudo ./mping -r 5 8.8.8.8
sudo ./mping -r 100 8.8.8.8,8.8.4.4
```