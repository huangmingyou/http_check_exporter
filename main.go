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
type U struct {
	Name    string        `yaml:"name"`
	Url     string        `yaml:"url"`
	Method  string        `yaml:"method"`
	Respons string        `yaml:"respons"`
	Query   string        `yaml:"query"`
	Timeout time.Duration `yaml:"timeout"`
}

type C struct {
	Thread  int `yaml:"thread"`
	Targets []U `yaml:",flow"`
}

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
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		//ExpectContinueTimeout: 10 * time.Second,
	}

	trace := &httptrace.ClientTrace{
		DNSStart: func(dsi httptrace.DNSStartInfo) { dns = time.Now() },
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			fmt.Printf("http_dns_time{name=\"%s\"} \t\t%v\n", t.Name, time.Since(dns))
			res_str = fmt.Sprintf("http_dns_time{name=\"%s\"} \t\t%d\n", t.Name, time.Since(dns))
		},
		TLSHandshakeStart: func() { tlsHandshake = time.Now() },
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			fmt.Printf("http_tls_handshake_time{name=\"%s\"} \t%v\n", t.Name, time.Since(tlsHandshake))
			res_str += fmt.Sprintf("http_tls_handshake_time{name=\"%s\"} \t%d\n", t.Name, time.Since(tlsHandshake))
		},

		ConnectStart: func(network, addr string) { connect = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			fmt.Printf("http_connect_time{name=\"%s\"} \t\t%v\n", t.Name, time.Since(connect))
			res_str += fmt.Sprintf("http_connect_time{name=\"%s\"} \t\t%d\n", t.Name, time.Since(connect))
		},

		GotFirstResponseByte: func() {
			fmt.Printf("http_firstbyte_time{name=\"%s\"} \t%v\n", t.Name, time.Since(start))
			res_str += fmt.Sprintf("http_firstbyte_time{name=\"%s\"} \t%d\n", t.Name, time.Since(start))
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	start = time.Now()
	resp, err := tr.RoundTrip(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	fmt.Printf("http_total_time{name=\"%s\"} \t\t%v\n", t.Name, time.Since(start))
	res_str += fmt.Sprintf("http_total_time{name=\"%s\"} \t\t%d\n", t.Name, time.Since(start))
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	matchv1 := 1
	var validID = regexp.MustCompile(t.Respons)
	matchv := validID.MatchString(string(body))
	if matchv {
		matchv1 = 0
	}
	fmt.Printf("http_content_match{name=\"%s\"} \t%d\n", t.Name, matchv1)
	res_str += fmt.Sprintf("http_content_match{name=\"%s\"} \t%d\n", t.Name, matchv1)
	c <- res_str
}

func Exporter(w http.ResponseWriter, r *http.Request) {
	ch1 := make(chan string)
	res2 := ""
	content, err := ioutil.ReadFile(cfgfile)
	if err != nil {
		log.Fatal(err)
	}
	c := C{}
	err1 := yaml.Unmarshal(content, &c)
	if err1 != nil {
		log.Fatalf("error: %v", err1)
	}
	for i := 0; i < len(c.Targets); i++ {
		go timeGet(c.Targets[i], ch1)
		res2 += <-ch1
	}
	fmt.Fprintf(w, res2)
}

func runcli() {
	ch1 := make(chan string)
	res2 := ""
	content, err := ioutil.ReadFile(cfgfile)
	if err != nil {
		log.Fatal(err)
	}
	c := C{}
	err1 := yaml.Unmarshal(content, &c)
	if err1 != nil {
		log.Fatalf("error: %v", err1)
	}
	for i := 0; i < len(c.Targets); i++ {
		go timeGet(c.Targets[i], ch1)
		res2 += <-ch1
	}
}
func main() {
	cfgPath, runmode, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	cfgfile = cfgPath
	if runmode == "web" {
		fmt.Println(cfgPath)
		http.HandleFunc("/metrics", Exporter)
		log.Fatal(http.ListenAndServe(":8080", nil))
	} else {
		runcli()
	}

}
