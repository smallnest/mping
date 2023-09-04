package main

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertAddrs(t *testing.T) {
	ip1 := "8.8.8.8"
	ip2 := "114.114.114.114"

	src := fmt.Sprintf("%s,%s", ip1, ip2)
	addrs, err := convertAddrs(src)
	assert.Nil(t, err)
	assert.EqualValues(t, src, strings.Join(addrs, ","))

	src = fmt.Sprintf("%s,%s", ip1, "www.bing.com")
	addrs, err = convertAddrs(src)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(addrs))

	ip := net.ParseIP(addrs[1])
	assert.NotNil(t, ip)
	t.Log(ip)
	// output: 202.89.233.100

	src = fmt.Sprintf("%s, %s", ip1, ip2)
	addrs, err = convertAddrs(src)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(addrs))
	assert.Equal(t, ip1, addrs[0])
	assert.Equal(t, ip2, addrs[1])

	src = "1.1.0.256, 256.256.256"
	addrs, err = convertAddrs(src)
	assert.NotNil(t, err)
	assert.Equal(t, 0, len(addrs))

	src = "www.bing.com.cn.xx"
	addrs, err = convertAddrs(src)
	assert.NotNil(t, err)
	assert.Equal(t, 0, len(addrs))

	src = "www.bing.com.cn/dont_with_uri"
	addrs, err = convertAddrs(src)
	assert.NotNil(t, err)
	assert.Equal(t, 0, len(addrs))
}
