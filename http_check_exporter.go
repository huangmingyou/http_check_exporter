package main

/*
读取config.yaml 文件
对yaml里面的site进行http检查
huangmingyou@gmail.com
2021.07
*/

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var cfgfile string
var metrics string

// 配置文件结构
type U struct {
	Name    string        `yaml:"name"`
	Url     string        `yaml:"url"`
	Method  string        `yaml:"method"`
	Respons string        `yaml:"respons"`
	Query   string        `yaml:"query"`
	Timeout time.Duration `yaml:"timeout"`
}

type C struct {
	Thread     int    `yaml:"thread"`
	Updatecron string `yaml:"updatecron"`
	Targets    []U    `yaml:",flow"`
}

var yc C

// 定义metrics
var (
	http_dns_time = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_dns_time",
			Help: "dns 连接耗时",
		},
		[]string{"name"},
	)
	http_tls_handshake_time = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_tls_handshake_time",
			Help: "ssl 握手耗时",
		},
		[]string{"name"},
	)
	http_connect_time = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_connect_time",
			Help: "http 连接耗时",
		},
		[]string{"name"},
	)
	http_firstbyte_time = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_firstbyte_time",
			Help: "接收到第一个字节耗时",
		},
		[]string{"name"},
	)
	http_total_time = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_total_time",
			Help: "http总共耗时",
		},
		[]string{"name"},
	)
	http_site_connect = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_site_connect",
			Help: "站点连通性",
		},
		[]string{"name"},
	)
	http_content_match = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_content_match",
			Help: "返回内容是否匹配",
		},
		[]string{"name"},
	)
)

func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory, not a normal file", path)
	}
	return nil
}

func ParseFlags() (string, error) {
	var configPath string

	flag.StringVar(&configPath, "config", "./config.yml", "path to config file")

	flag.Parse()
	if err := ValidateConfigPath(configPath); err != nil {
		return "", err
	}
	return configPath, nil
}

func timeGet(t U, c chan string) {
	var res_str string
	var tmp_str string
	tmp_str = fmt.Sprintf("http_site_connect{name=\"%s\"} 0\n", t.Name)
	req, _ := http.NewRequest(t.Method, t.Url, nil)
	if "POST" == t.Method {
		req, _ = http.NewRequest(t.Method, t.Url, strings.NewReader(t.Query))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Content-Length", strconv.Itoa(len(t.Query)))
	}
	var start, connect, dns, tlsHandshake time.Time
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   t.Timeout * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        10,
		IdleConnTimeout:     10 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		//ExpectContinueTimeout: 10 * time.Second,
	}

	trace := &httptrace.ClientTrace{
		DNSStart: func(dsi httptrace.DNSStartInfo) { dns = time.Now() },
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			http_dns_time.With(prometheus.Labels{"name": t.Name}).Set(float64(time.Since(dns) / time.Nanosecond))
		},
		TLSHandshakeStart: func() { tlsHandshake = time.Now() },
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			http_tls_handshake_time.With(prometheus.Labels{"name": t.Name}).Set(float64(time.Since(tlsHandshake) / time.Nanosecond))
		},

		ConnectStart: func(network, addr string) { connect = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			http_connect_time.With(prometheus.Labels{"name": t.Name}).Set(float64(time.Since(connect) / time.Nanosecond))
		},

		GotFirstResponseByte: func() {
			http_firstbyte_time.With(prometheus.Labels{"name": t.Name}).Set(float64(time.Since(start) / time.Nanosecond))
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	start = time.Now()
	resp, err := tr.RoundTrip(req)
	if err != nil {
		fmt.Println(err)
		http_site_connect.With(prometheus.Labels{"name": t.Name}).Set(1)
		fmt.Println("错误结束",t.Name)
		c <- tmp_str
		return
	}
	defer resp.Body.Close()
	http_total_time.With(prometheus.Labels{"name": t.Name}).Set(float64(time.Since(start) / time.Nanosecond))
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		http_site_connect.With(prometheus.Labels{"name": t.Name}).Set(1)
		fmt.Println("错误结束",t.Name)
		c <- tmp_str
		return
	}
	http_site_connect.With(prometheus.Labels{"name": t.Name}).Set(0)

	matchv1 := 1
	var validID = regexp.MustCompile(t.Respons)
	matchv := validID.MatchString(string(body))
	if matchv {
		matchv1 = 0
	}
	http_content_match.With(prometheus.Labels{"name": t.Name}).Set(float64(matchv1))
	res_str += tmp_str
		fmt.Println("正常结束",t.Name)
	c <- res_str
}

func Exporter(w http.ResponseWriter, r *http.Request) {
	ch1 := make(chan string)
	res2 := ""
	for i := 0; i < len(yc.Targets); i++ {
		go timeGet(yc.Targets[i], ch1)
		res2 += <-ch1
	}
	fmt.Fprintf(w, res2)
}

func runcli() {
	ch1 := make(chan string)
	res2 := ""
	for i := 0; i < len(yc.Targets); i++ {
		go timeGet(yc.Targets[i], ch1)
		res2 += <-ch1
	}
	metrics = res2
	//	fmt.Println(time.Now())
	//fmt.Println(res2)
	fmt.Println("执行完成")
}
func regmetrics() {
	prometheus.MustRegister(http_dns_time)
	prometheus.MustRegister(http_tls_handshake_time)
	prometheus.MustRegister(http_connect_time)
	prometheus.MustRegister(http_firstbyte_time)
	prometheus.MustRegister(http_total_time)
	prometheus.MustRegister(http_site_connect)
	prometheus.MustRegister(http_content_match)
}
func main() {
	regmetrics()
	cfgPath, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	content, err := ioutil.ReadFile(cfgPath)
	err1 := yaml.Unmarshal(content, &yc)
	if err1 != nil {
		log.Fatalf("error: %v", err1)
	}
	cfgfile = cfgPath
	// cron job
	cjob := cron.New()
	cjob.AddFunc(yc.Updatecron, runcli)
	cjob.Start()
	//
	runcli()
	//

	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(":8080", nil))

}
