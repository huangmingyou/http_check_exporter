
build
=====
> go mod init  http_check_exporter

> go mod tidy

> go build http_check_exporter.go


run
===

> ./http_check_exporter -config ./config.yml -mode web

> test

> curl 127.0.0.1:8080/metrics

or 

> ./http_check_exporter -config ./config.yml -mode cli


config
======

在yml文件添加站点

检查逻辑：

通过method的方式请求url，query是请求参数，可选。

在返回结果中匹配respons中的字符串，返回匹配结果。

> \- name: api

>     url: http://api.example.cn/api

>     method: POST

>     respons: "you want"

>     query: "url=http%3A%2F%2Fapi.example.com%2Fapi1&p1=abc"

>     timeout: 10



output
=======
> dns 查询时间
> http_dns_time{name="test1"} 		44095447

> connect 时间
> http_connect_time{name="test1"} 		10770413

> tls 握手时间
> http_tls_handshake_time{name="test1"} 	199979338

> 收到第一个字节的时间	
> http_firstbyte_time{name="test1"} 	268094039

> 总时间
> http_total_time{name="test1"} 		268342993

> 结果是否匹配respons
> http_content_match{name="test1"} 	1

