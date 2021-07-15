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

func ParseFlags() (string, string, error) {
	var configPath string
	var mode string

	flag.StringVar(&configPath, "config", "./config.yml", "path to config file")
	flag.StringVar(&mode, "mode", "cli", "run mode, cli or web")

	flag.Parse()
	if err := ValidateConfigPath(configPath); err != nil {
		return "", "", err
	}
	return configPath, mode, nil
}

func timeGet(t U, c chan string) {
	var res_str string
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
			res_str = fmt.Sprintf("http_dns_time{name=\"%s\"} \t\t%d\n", t.Name, time.Since(dns))
		},
		TLSHandshakeStart: func() { tlsHandshake = time.Now() },
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			res_str += fmt.Sprintf("http_tls_handshake_time{name=\"%s\"} \t%d\n", t.Name, time.Since(tlsHandshake))
		},

		ConnectStart: func(network, addr string) { connect = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			res_str += fmt.Sprintf("http_connect_time{name=\"%s\"} \t\t%d\n", t.Name, time.Since(connect))
		},

		GotFirstResponseByte: func() {
			res_str += fmt.Sprintf("http_firstbyte_time{name=\"%s\"} \t%d\n", t.Name, time.Since(start))
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	start = time.Now()
	resp, err := tr.RoundTrip(req)
	if err != nil {
		fmt.Println(err)
		c <- ""
		return
	}
	defer resp.Body.Close()
	res_str += fmt.Sprintf("http_total_time{name=\"%s\"} \t\t%d\n", t.Name, time.Since(start))
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		c <- ""
		return
	}

	matchv1 := 1
	var validID = regexp.MustCompile(t.Respons)
	matchv := validID.MatchString(string(body))
	if matchv {
		matchv1 = 0
	}
	res_str += fmt.Sprintf("http_content_match{name=\"%s\"} \t%d\n", t.Name, matchv1)
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
	fmt.Println(res2)
}
func main() {
	cfgPath, runmode, err := ParseFlags()
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
	if runmode == "web" {
		//init data
		runcli()
		//
		http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, metrics)
		})
		log.Fatal(http.ListenAndServe(":8080", nil))
	} else {
		runcli()
	}

}
