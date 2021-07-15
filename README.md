# http_check_exporter

## 简介

http_check_exporter 是一个prometheus exporter 。 同时也执行cli模式运行。直接输出检查结果。

程序从yml文件读取待检查站点的配置，yml里面需要配置站点url, http请求方式(HEAD,GET,POST等),

以及可选的参数。程序会返回请求站点的dns查询时间，tls握手时间，接收到第一个字节以及总的时间。

同时，会对返回结果进行字符串匹配。


## 编译

获取代码以后在代码目录执行:

  ```
  go mod init http_check_exporter
  go mod tidy
  GOOS=linux GOARCH=amd64 go build http_check_exporter.go
  ```



## 配置

  ```yaml
  ---
  thread: 10
  # crontab 格式的配置，定期更新数据"
  updatecron: "* * * * *"
  targets:
   - name: test1
     url: http://127.0.0.1/hmy.pub
     method: GET
     respons: "AAAA"
     query: nil
     timeout: 10
   - name: baidu
     url: https://www.baidu.cn
     #请求方式
     method: POST
     respons: vphoto
     #参数
     query: testdata
     #connect 超时时间
     timeout: 10
  ```

## 运行

  ```bash
  # 命令行方式执行，直接输出结果
  ./http_check_exporter -config ./config.yml -mode cli
  # web 方式，以exporter方式运行，访问http://youip:8080/metrics 获取结果
  ./http_check_exporter -config ./config.yml -mode cli
  ```
  

## 执行逻辑

周期访问站点缓存结果，避免访问/metrics的时候延迟过高。
