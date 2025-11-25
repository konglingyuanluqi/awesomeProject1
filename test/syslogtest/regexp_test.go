package syslog

import (
	"encoding/json"
	"fmt"
	"github.com/araddon/dateparse"
	"github.com/miekg/dns"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	syslog_prase "syslogtest/syslog_parse"
	"testing"
	"time"

	systemSyslog "log/syslog"
)

func TestRegexp1(t *testing.T) {
	str := `Jul 13 18:15:39 houxinrui-VirtualBox unbound: [7652:0] info: ::1 daisy.ubuntu.com. A IN`

	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`info: (?P<client_ip>.+) (?P<query_name>[\w.-]+) (?P<query_type>\w+) (?P<query_class>\w+)`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	fmt.Println(groupNames)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)
}

func TestRegexpZdns(t *testing.T) {
	str := `host-10-20-89-125 18-May-2016 18:30:59.938 client 127.0.0.1 36031: view default: dns.zdns.cn IN A NOERROR+ NS NE NT ND NC H`

	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`host-(?P<server_ip_type1>.*?) (?P<datetime>.+) client (?P<client_ip>.+) (?P<client_port>.+): view .+: (?P<query_name>.+) IN (?P<query_type>\w+) .+`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	fmt.Println(groupNames)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)
}

func TestRegexpZdns2(t *testing.T) {
	str := `localhost 22-Oct-2024 16:31:21.088 client 10.163.160.223 59087: view xn--v6qs0s5pl0nx: local.id.seewo.com IN A NXDOMAIN + NS NE NT ND NC H 0 0 0 0.000000 NW NA NA U NA 8`
	str = `localhost 22-Oct-2024 16:31:21.088 client 10.165.5.67 59091: view xn--v6qs0s5pl0nx: p3-sign.douyinpic.com IN TYPE65 NOERROR + NS NE NT`
	str = `localhost 19-Jun-2025 15:41:14.843 client 10.215.25.82 52081: view View_C2: nitropay-90.b-cdn.net IN TYPE65 NOERROR + NS NE NT ND NC L 0 0 0 0.000000 NW NA NA B FW source client NA 143950`
	str = `localhost  14-Oct-2025 11:17:01.154 client 210.42.122.221 56585: view xn--idc-7w2et6oo83a: msgp.whu.edu.cn IN A NOERROR + NS E NT ND NC H 0 0 0 0.000000 NW NA NA U NA source client NA 32`
	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`[\w ]+ (?P<datetime>.+) client (?P<client_ip>.+) (?P<client_port>.+): view .+: (?P<query_name>.+) IN (?P<query_type>\w+) (?P<rcode>\w+) .+`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	fmt.Println(groupNames)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)
}
func TestRegexp2(t *testing.T) {
	str := `39.144.81.88|beacon.sina.com.cn.|20210324172228|111.13.134.212;39.156.6.183|0|2|||117.131.225.231|0`

	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`^(?P<client_ip>.*?)\|(?P<query_name>.*?)\|(?P<datetime>.*?)\|.*?\|.*?\|(?P<query_type>.*?)\|`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)
}

func TestRegexp3(t *testing.T) {
	//str := `response: { "handling_time": 52, "client": "2001:250:4808:1021:dd82:f27b:e374:9793", "client_port": 62604, "qname": "ipv6.msftconnecttest.com", "view": "policy_view_5", "type": "A", "result": "OK", "ret": [ "ipv6.msftconnecttest.com. 2770\tIN\tCNAME\tv6ncsi.msedge.net.", "v6ncsi.msedge.net.\t66\tIN\tCNAME\tncsi.6-c-0003.c-msedge.net.", "ncsi.6-c-0003.c-msedge.net. 66\tIN\tCNAME\t6-c-0003.c-msedge.net." ] }`
	//str := `HUAUDDIC-DNS|18-May-2022 17:12:08.055 172.21.0.43#34695 view_LOCAL nxrrset content.rconfig.qq.com IN TYPE65 (10.10.10.10)`
	// 使用命名分组，显得更清晰

	//if t, ok := syslog.StringTypeNumberToType["TYPE65"]; ok {
	//fmt.Println(t)
	//}

	//str := ` HUAUDDIC-DNS|18-May-2022 17:12:07.947 192.168.138.73#59406 view_LOCAL success notice.wps.cn IN A (10.10.10.8)`
	//str2 := ` HUAUDDIC-DNS|18-May-2022 17:12:08.051 192.168.27.60#50292 view_LOCAL success client.wns.windows.com IN A (10.10.10.10)`
	//str3 := ` HUAUDDIC-DNS|18-May-2022 17:12:07.948 192.168.138.73#53197 view_LOCAL success roaming.wps.cn IN A (10.10.10.8)`
	//str4 := ` HUAUDDIC-DNS|18-May-2022 17:12:07.950 192.168.138.73#54194 view_LOCAL success pay.wps.cn IN A (10.10.10.8)`
	//str5 := ` HUAUDDIC-DNS|18-May-2022 17:12:08.054 192.168.100.145#62394 view_LOCAL success dl.vas.wpscdn.cn IN A (10.10.10.10)`
	//str6 := ` HUAUDDIC-DNS|18-May-2022 17:12:08.054 192.168.100.145#60582 view_LOCAL success dl.vas.wpscdn.cn IN A (10.10.10.10)`
	//str7 := ` HUAUDDIC-DNS|18-May-2022 17:12:07.951 192.168.138.73#64260 view_LOCAL success geo.wps.cn IN A (10.10.10.8)`
	//str8 := ` HUAUDDIC-DNS|18-May-2022 17:12:07.951 192.168.138.73#57697 view_LOCAL success account.wps.cn IN A (10.10.10.8)`
	//str9 := ` HUAUDDIC-DNS|18-May-2022 17:12:07.951 192.168.138.73#61092 view_LOCAL success vip.wps.cn IN A (10.10.10.8)`
	str := ` HUAUDDIC-DNS|18-May-2022 17:12:08.055 172.21.0.43#34695 view_LOCAL nxrrset content.rconfig.qq.com IN TYPE65 (10.10.10.10)`
	//str11 := ` HUAUDDIC-DNS|18-May-2022 17:12:07.952 192.168.138.73#57787 view_LOCAL success vipapi.wps.cn IN A (10.10.10.8)`
	//str12 := ` HUAUDDIC-DNS|18-May-2022 17:12:08.056 192.168.78.101#63425 view_LOCAL success 1min.pcfg.cache.wpscdn.cn IN A (10.10.10.10)`

	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	//re := regexp.MustCompile(`\"client\": \"(?P<client_ip>.+)\", \"client_port\".+\"qname\": \"(?P<query_name>.+)\", \"view\".+\"type\": \"(?P<query_type>\w+)\", \"result\"`)
	re := regexp.MustCompile(`.+ .+ (?P<client_ip>.+)#.+ .+ .+ (?P<query_name>.+) (?P<query_class>\w+) (?P<query_type>\w+) .+`)

	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	fmt.Println(groupNames)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)
}

func TestRegexp4(t *testing.T) {
	str := `202.102.224.68|53|42.232.99.142|63686|45241|pgdt.gtimg.cn|A|A_182.118.63.196;A_42.236.95.37;A_42.236.95.33;A_42.236.95.36;A_182.118.63.200;A_42.236.95.35;A_42.236.95.34|20160505131555.514|0|r`
	str = `56.56.56.56|53|160.66.227.8|24553|13511|J7.aLJ.JY.vvY.4n-syy8.s81s|PTR||20230320123004.001|2|r`
	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`(?P<server_ip>.*?)\|(?P<server_port>[0-9]*?)\|(?P<client_ip>.*?)\|(?P<client_port>[0-9]*?)\|(?P<transaction_id>[0-9]*?)\|(?P<query_name>.*?)\|(?P<query_type>\w+)\|(?P<rdata_type1>.*?)\|(?P<datetime_layout>.*?)\|(?P<rcode>[0-9]*?)\|r`)

	str = `18-May-2016 18:30:59.938 client 127.0.0.1 36031: view default: dns.zdns.cn IN A NOERROR + NS NE NT ND NC H`
	re = regexp.MustCompile(`(?P<datetime>.+) client (?P<client_ip>.+) (?P<client_port>.+): view .+: (?P<query_name>.+) IN (?P<query_type>\w+) (?P<rcode>\w+) [+-] .+`)

	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	fmt.Println(groupNames)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	DnsMsg := new(dns.Msg)
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	clientIP := ""
	rcode := dns.RcodeSuccess
	clientPort := 0
	serverIP := ""
	serverPort := 0
	queryName := ""
	queryType := dns.TypeA
	queryClass := uint16(dns.ClassINET)

	for i, name := range groupNames {
		switch name {
		case "client_ip":
			clientIP = match[i]
		case "client_port":
			i, err := strconv.Atoi(match[i])
			if err == nil {
				clientPort = i
			} else {
				println(err.Error())
			}

		case "server_ip":
			serverIP = match[i]
		case "server_port":
			i, err := strconv.Atoi(match[i])
			if err == nil {
				serverPort = i
			} else {
				println(err.Error())
			}
		case "query_name":
			queryName = match[i]
		case "query_class":
			if t, ok := dns.StringToClass[match[i]]; ok {
				queryClass = t
			} else {
				queryType = dns.ClassINET
			}
		case "rdata_type1":
			fmt.Println(match[i])
			rdata := match[i]
			res1 := strings.Split(rdata, ";")
			for _, res2 := range res1 {
				res3 := strings.Split(res2, "_")
				fmt.Println(res3)
			}
			fmt.Println(res1)

		case "datetime":
			t, err := dateparse.ParseLocal(match[i])
			fmt.Println("==date")
			if err == nil {
				fmt.Println(t.Date())
			} else {
				println(err.Error())
			}
		case "datetime_layout":
			loc, _ := time.LoadLocation("Asia/Shanghai")
			t, err := time.ParseInLocation("20060102150405.999", match[i], loc)
			fmt.Println("==date")
			if err == nil {
				fmt.Println(t.String())
			} else {
				println(err.Error())
			}
		case "query_type":
			if t, ok := dns.StringToType[match[i]]; ok {
				queryType = t
			} else if t, ok := syslog_prase.StringNumberToType[match[i]]; ok {
				queryType = t
			} else if t, ok := syslog_prase.StringTypeNumberToType[match[i]]; ok {
				queryType = t
			} else {
				queryType = dns.TypeA
			}
		case "rcode":
			if t, ok := dns.StringToRcode[match[i]]; ok {
				rcode = t
			} else {
				rcode = dns.RcodeSuccess
			}
		}
	}
	address := net.ParseIP(clientIP)
	if address == nil {
		return
	} else {
		fmt.Println(address.String())
	}

	_, ok := dns.IsDomainName(queryName)
	if ok {
		DnsMsg.SetQuestion(dns.Fqdn(queryName), queryType)
	} else {
		return
	}
	DnsMsg.Question[0].Qclass = queryClass
	fmt.Println(DnsMsg)

	fmt.Println(rcode)
	fmt.Println(clientIP)
	fmt.Println(clientPort)
	fmt.Println(serverIP)
	fmt.Println(serverPort)
}

func TestRegexp5(t *testing.T) {
	re := regexp.MustCompile(`(?P<server_ip>.*?)\|(?P<server_port>[0-9]*?)\|(?P<client_ip>.*?)\|(?P<client_port>[0-9]*?)\|(?P<transaction_id>[0-9]*?)\|(?P<query_name>.*?)\|(?P<query_type>\w+)\|(?P<rdata_type1>.*?)\|(?P<datetime_layout>.*?)\|(?P<rcode>[0-9]*?)\|r`)
	str2 := `52.72.72.72|53|160.66.45.90|48152|17539|1Kkk-8tm1-kv-cnP.yoKP4ncyn.com|AAAA|(CNAME)_1Kkk-8tm1-kv-cnP.yoKP4ncyn.com.wAyXA.com|20230320123004.002|0|r`
	str2 = `56.56.56.56|53|160.66.227.8|24553|13511|J7.aLJ.JY.vvY.4n-syy8.s81s|PTR||20230320123004.001|2|r`

	parse := syslog_prase.New()
	parse.SetTimeLayOut("20060102150405.999", "Asia/Shanghai")

	pb, err := parse.ParseRegexp(re, str2)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}

func TestRegexp6(t *testing.T) {
	sysLog, err := systemSyslog.Dial("udp", "localhost:5141", systemSyslog.LOG_INFO, "golang")
	if err != nil {

		log.Fatal(err)
	}
	_ = sysLog.Emerg("52.72.72.72|53|160.66.45.90|48152|17539|1Kkk-8tm1-kv-cnP.yoKP4ncyn.com|AAAA|(CNAME)_1Kkk-8tm1-kv-cnP.yoKP4ncyn.com.wAyXA.com|20230320123004.002|0|r")

}

func TestRegexp7(t *testing.T) {
	str := `Ailog{"rawEvent":"2024/10/21 19:00:35 10FC PACKET  0000020D6E8F03E0 UDP Rcv 192.170.65.34   3af6   Q [0001   D   NOERROR] A      (18)conn-service-cn-04(10)allawntech(3)com(0)"}`
	//str = `Ailog{"rawEvent":"2024/10/21 19:02:27 10FC PACKET  0000020D0C784CE0 UDP Snd 192.168.113.210 d1a7 R Q [8385 A DR NXDOMAIN] PTR    (3)254(3)113(3)168(3)192(7)in-addr(4)arpa(0)"}`
	str = `Ailog{"rawEvent":"2024/10/21 19:02:27 10DC PACKET  0000020D6CF7D8D0 UDP Snd 192.168.0.246   6aca R Q [8281   DR SERVFAIL] AAAA   (11)FSSCPRDAPP2(0)"}`
	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`^Ailog\{\"rawEvent\":\"(?P<datetime>.*?) +[0-9A-Fa-f]{4} PACKET .+ (UDP|TCP) +(Snd|Rcv) +(?P<client_ip>.+?) +(?P<client_port_hex>.+?) .+ \[.+ .+ +(?P<rcode>\w+?)] +(?P<query_type>\w+?) +(?P<query_name_type1>.*?)\"}`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	parse := syslog_prase.New()
	pb, err := parse.ParseRegexp(re, str)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}

func TestRegexp8(t *testing.T) {
	str := `2024-10-22 15:39:20 zdns info  client 169.254.108.198 63457: view default: query (cache) 'isatap.lan/A/IN' denied^J`

	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`^(?P<datetime>.*?) zdns info  client (?P<client_ip>.*?) (?P<client_port>[0-9]*?): view .+ '(?P<query_name>.+)/(?P<query_type>.*?)/IN'`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)
}

func TestRegexp9(t *testing.T) {
	str := `2024-12-26 09:57:49|10.18.30.61|qq.com|1|`

	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`(?P<datetime>.*?)\|(?P<client_ip>.*?)\|(?P<query_name>.*?)\|(?P<query_type>.*?)\|`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	parse := syslog_prase.New()
	pb, err := parse.ParseRegexp(re, str)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}

//网瑞达

func TestRegexp10(t *testing.T) {
	str := `client @0x7f0981f4e910 10.105.0.154#56317 (static-s.iqiyi.com): view policy_view_1: query: static-s.iqiyi.com IN TYPE65 + (202.113.112.50)`
	str = `Jun 13 11:20:04 172.18.10.3 named[0]: client @0x0 10.0.220.48#0 (pull-lls-q5.douyincdn.com.): view policy_view_10: query: pull-lls-q5.douyincdn.com. IN A + (172.18.10.3)`
	str = "client @0x0 10.3.241.187#0 (ice-stunserver-v6.xdrtc.com.): view policy_view_10: query: ice-stunserver-v6.xdrtc.com. IN A + (172.18.10.3)"
	str = "12-Sep-2025 17:03:56.635 queries: client @0x7f22f404b620 223.2.43.8#23253 (api.miwifi.com): view ext2: query: api.miwifi.com IN AAAA + (202.119.104.31)" // 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`(?P<datetime>.*?) queries: client .+ (?P<client_ip>.*?)#(?P<client_port>[0-9]*?) \((?P<query_name>.*?)\): view .+ query: .+ IN (?P<query_type>.*?) .+ \((?P<server_ip>.*?)\)`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	parse := syslog_prase.New()
	pb, err := parse.ParseRegexp(re, str)
	//fmt.Println(re)
	//fmt.Println(str)
	//(?P<datetime>.*?) queries: client .+ (?P<client_ip>.*?)#(?P<client_port>[0-9]*?) \((?P<query_name>.*?)\): view .+ query: .+ IN (?P<query_type>.*?) .+ \((?P<server_ip>.*?)\)
	//12-Sep-2025 17:03:56.635 queries: client @0x7f22f404b620 223.2.43.8#23253 (api.miwifi.com): view ext2: query: api.miwifi.com IN AAAA + (202.119.104.31)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}

// 冰川DNS
func TestRegexp11(t *testing.T) {
	str := `1 2025-05-16T11:52:26 GlacierDNS(dnscache): 223.2.44.246:57110->202.119.104.31:53,url:pc-mon.zijieapi.com[19],dns_id:4896,type:0x0001,responds with gdns cache,line:1(M-5M-gM-PM-E M-x ),order:0x01020400,answer with:120.220.191.114,171.15.110.180,180.97.246.124,180.97.248.210,223.86.122.184,223.113.138.204.^J`

	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`1 (?P<datetime>.*?) GlacierDNS.dnscache.: (?P<client_ip>.*?):(?P<client_port>[0-9]*?)->(?P<server_ip>.*?):(?P<server_port>[0-9]*?),url:(?P<query_name>.*?)\[.+`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	parse := syslog_prase.New()
	pb, err := parse.ParseRegexp(re, str)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}

// 冰川DNS
func TestRegexp12(t *testing.T) {
	str := `dnscache: 1747381754 2025-05-16 15:49:14 queries:api.simpleallowcopy.com IN A from client 202.119.106.92#58830,dns_server 202.119.107.33:53 domain nocache forward ISP dns,set MARK:0x01030204`

	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`dnscache: [0-9]*? (?P<datetime>.*?) queries:(?P<query_name>.*?) IN (?P<query_type>.*?) from client (?P<client_ip>.*?)#(?P<client_port>[0-9]*?),dns_server (?P<server_ip>.*?):(?P<server_port>[0-9]*?) .+`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	parse := syslog_prase.New()
	pb, err := parse.ParseRegexp(re, str)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}

//qianxin
//

func TestRegexp13(t *testing.T) {
	str := `DNS威胁检测与日志分析系统|*0|*2025-05-29 11:58:07|*|*ABCD:EF01:2345:6789:ABCD:EF01:2345:6789|*usw2-register-appattest-prod-aws-prod.apple.com|*HTTPS|*NOERROR|*0`

	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`DNS威胁检测与日志分析系统\|\*0\|\*(?P<datetime>.*?)\|\*(?P<client_ip>.*?)\|\*\|\*(?P<query_name>.*?)\|\*(?P<query_type>.*?)\|\*(?P<rcode>\w+)\|`)
	//re := regexp.MustCompile(`DNS威胁检测与日志分析系统\|\*0\|\*(?P<datetime>.*?)\|\*\|\*(?P<client_ip>.*?)\|\*(?P<query_name>.*?)\|\*(?P<query_type>.*?)\|\*(?P<rcode>\w+)\|`)

	//re := regexp.MustCompile(`dnscache: [0-9]*? (?P<datetime>.*?) queries:(?P<query_name>.*?) IN (?P<query_type>.*?) from client (?P<client_ip>.*?)#(?P<client_port>[0-9]*?),dns_server (?P<server_ip>.*?):(?P<server_port>[0-9]*?) .+`)

	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	parse := syslog_prase.New()
	pb, err := parse.ParseRegexp(re, str)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}

// 木云DNS
func TestRegexp14(t *testing.T) {
	str := `2025-06-04T15:26:36.926 queries: info: client @0x7f9a08001018 172.24.65.32#12345 (h5hosting-drcn.dbankcdn.cn): view v8_view: query: h5hosting-drcn.dbankcdn.cn IN A +E(0)`

	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`(?P<datetime>.*?) queries: info: client .+ (?P<client_ip>.*?)#(?P<client_port>[0-9]*?) .+: view .+ query: (?P<query_name>.*?) IN (?P<query_type>.*?) .+`)
	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	parse := syslog_prase.New()
	pb, err := parse.ParseRegexp(re, str)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}

// 启明星辰负载
func TestRegexp15(t *testing.T) {
	str := `SerialNum=001021002006822212206337 GenTime="2025-09-16 15:15:16" SrcIP=172.16.30.48 DstIP=172.16.30.71 Content="query: access-open.quark.cn IN A +E" EvtCount=1`
	str = `SerialNum=001021002006822212206337 GenTime="2025-09-16 15:28:55" SrcIP=172.31.24.11 Content="query failed (SERVFAIL) for rdate.darkorb.net/IN/AAAA" EvtCount=1`
	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`GenTime=\"(?P<datetime>.*?)\" SrcIP=(?P<client_ip>.*?) DstIP=(?P<server_ip>.*?) Content=\"query: (?P<query_name>.*?) IN (?P<query_type>.*?) \+`)
	re = regexp.MustCompile(`GenTime=\"(?P<datetime>.*?)\" SrcIP=(?P<client_ip>.*?) Content=\"query failed \((?P<rcode>\w+)\) for (?P<query_name>.*?)/IN/(?P<query_type>.*?)\"`)

	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	parse := syslog_prase.New()
	pb, err := parse.ParseRegexp(re, str)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}

// 启明星辰负载
func TestRegexp17(t *testing.T) {
	str := `SerialNum=001021002006822212206337 GenTime="2025-09-16 15:15:16" SrcIP=172.16.30.48 DstIP=172.16.30.71 Content="query: access-open.quark.cn IN A +E" EvtCount=1`
	str = `{"family":"IPv4","protocol":"UDP","query-ip":"10.40.17.81","query-port":"45006","response-ip":"10.10.5.65","response-port":"53","ip-defragmented":false,"tcp-reassembled":false},"dns":{"length":32,"opcode":0,"rcode":"NOERROR","qname":"v2.kwaicdn.com","qtype":"A","flags":{"qr":false,"tc":false,"aa":false,"ra":false,"ad":false},"resource-records":{"an":[],"ns":[],"ar":[]},"malformed-packet":false},"edns":{"udp-size":0,"rcode":0,"version":0,"dnssec-ok":0,"options":[]},"dnstap":{"operation":"CLIENT_QUERY","identity":"dns-manager","version":"lighting 2.1.4","timestamp-rfc3339ns":"2025-10-23T04:20:29.974240599Z","latency":"0.000000","extra":"-"}}`
	str = `{"family":"IPv4","protocol":"UDP","query-ip":"10.40.2.34","query-port":"63493","response-ip":"10.10.5.65","response-port":"53","ip-defragmented":false,"tcp-reassembled":false},"dns":{"length":30,"opcode":0,"rcode":"NOERROR","qname":"api.weibo.cn","qtype":"HTTPS","flags":{"qr":false,"tc":false,"aa":false,"ra":false,"ad":false},"resource-records":{"an":[],"ns":[],"ar":[]},"malformed-packet":false},"edns":{"udp-size":0,"rcode":0,"version":0,"dnssec-ok":0,"options":[]},"dnstap":{"operation":"CLIENT_QUERY","identity":"dns-manager","version":"lighting 2.1.4","timestamp-rfc3339ns":"2025-10-23T04:32:41.540576528Z","latency":"0.000000","extra":"-"}}`
	// 使用命名分组，显得更清晰
	//re := regexp.MustCompile(`(?P<name>[a-zA-Z]+)`)
	re := regexp.MustCompile(`.+\"query-ip\":\"(?P<client_ip>.*?)\",\"query-port\":\"(?P<client_port>.+?)\",\"response-ip\":\"(?P<server_ip>.*?)\".+\"rcode\":\"(?P<rcode>\w+)\",\"qname\":\"(?P<query_name>.*?)\",\"qtype\":\"(?P<query_type>.*?)\"`)
	//re = regexp.MustCompile(`".+\"query-ip\":\"(?P<client_ip>.*?)\".+\"qname\":\"(?P<query_name>.*?)\".+`)

	match := re.FindStringSubmatch(str)

	groupNames := re.SubexpNames()

	fmt.Printf("%v, %v, %d, %d\n", match, groupNames, len(match), len(groupNames))

	result := make(map[string]string)

	// 转换为map
	for i, name := range groupNames {
		if i != 0 && name != "" { // 第一个分组为空（也就是整个匹配）
			result[name] = match[i]
		}
	}
	//
	prettyResult, _ := json.MarshalIndent(result, "", "  ")
	//
	fmt.Printf("%s\n", prettyResult)

	parse := syslog_prase.New()
	pb, err := parse.ParseRegexp(re, str)
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println(pb.String())
	}
}
