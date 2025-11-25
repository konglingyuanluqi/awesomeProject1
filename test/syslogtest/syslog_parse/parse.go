package syslog_prase

import (
	"errors"
	"github.com/araddon/dateparse"
	"net"
	"regexp"
	"strconv"
	"strings"
	"syslogtest/dns360protocol"
	"time"

	"github.com/miekg/dns"
)

//var log = clog.NewWithPlugin("lds_input_syslog")

//var loc, _ = time.LoadLocation("Asia/Shanghai")

type Parse struct {
	timeLayOut string
	loc        *time.Location
}

func New() *Parse {
	p := &Parse{}
	return p
}

func (s *Parse) SetTimeLayOut(layout, Location string) error {
	s.timeLayOut = layout
	loc, err := time.LoadLocation(Location)
	if err != nil {
		return err
	}
	s.loc = loc
	return nil
}

// ParseRegexp Handle 处理消息
func (s *Parse) ParseRegexp(re *regexp.Regexp, content string) (*dns360protocol.DnsMessage, error) {
	//defer func() {
	//	// In case the user doesn't enable error plugin, we still
	//	// need to make sure that we stay alive up here
	//	if rec := recover(); rec != nil {
	//		var buf [4096]byte
	//		log.Errorf("Recovered from panic in server: %q ====> %s\n", rec, string(buf[:runtime.Stack(buf[:], false)]))
	//		vars.Panic.Inc()
	//	}
	//}()
	if re == nil || len(content) == 0 {
		return nil, errors.New("parse parameter error")
	}
	pb := &dns360protocol.DnsMessage{ServerType: 9}
	pb.Tnow = uint32(time.Now().Unix())

	match := re.FindStringSubmatch(content)
	if match == nil {
		return nil, errors.New("parse not match")
	}
	groupNames := re.SubexpNames()

	for i, name := range groupNames {
		switch name {
		case "client_ip":
			pb.ClientAddress = match[i]
		case "server_ip":
			pb.ServerAddress = match[i]
		case "server_ip_type1": //1-1-1-1
			pb.ServerAddress = strings.ReplaceAll(match[i], "-", ".")
		case "client_port":
			if i, err := strconv.Atoi(match[i]); err == nil {
				pb.ClientPort = uint32(i)
			}
		case "client_port_hex":
			if decimalValue, err := strconv.ParseInt(match[i], 16, 64); err == nil {
				pb.ClientPort = uint32(decimalValue)
			}
		case "server_port":
			if i, err := strconv.Atoi(match[i]); err == nil {
				pb.ServerPort = uint32(i)
			}
		case "query_name": //www.baidu.com
			pb.FirstQueryName = GetQNameTrimZone(match[i])
		case "query_name_type1": // (8)dc1-file(3)ksn(14)kaspersky-labs(3)com(0)
			pb.FirstQueryName = GetQNameTrimZone(ParseDomainType1(match[i]))
		case "query_class":
			if t, ok := dns.StringToClass[match[i]]; ok {
				pb.FirstClass = uint32(t)
			}
		case "datetime":
			t, err := dateparse.ParseLocal(match[i])
			if err == nil {
				pb.Tnow = uint32(t.Unix())
			}
		case "datetime_unix":
			utime, err := strconv.Atoi(match[i])
			if err == nil {
				pb.Tnow = uint32(utime)
			}
		case "datetime_layout":
			if s.loc == nil || len(s.timeLayOut) == 0 {
				return nil, errors.New("init datetime layout error")
			}
			if t, err := time.ParseInLocation(s.timeLayOut, match[i], s.loc); err == nil {
				pb.Tnow = uint32(t.Unix())
			} else {
				return nil, errors.New("parse datetime error: " + err.Error())
			}
		case "query_type":
			if t, ok := dns.StringToType[match[i]]; ok {
				pb.FirstType = uint32(t)
			} else if t, ok := StringNumberToType[match[i]]; ok {
				pb.FirstType = uint32(t)
			} else if t, ok := StringTypeNumberToType[match[i]]; ok {
				pb.FirstType = uint32(t)
			}
		case "rdata_type1":

			match[i] = strings.Replace(match[i], "(", "", -1)
			match[i] = strings.Replace(match[i], ")", "", -1)
			rrList := strings.Split(match[i], ";")

			rrName := pb.FirstQueryName
			for _, rrValue := range rrList {
				rr := strings.SplitN(rrValue, "_", 2)
				if len(rr) == 2 {
					rrType := getDNSType(rr[0])
					rrPb := &dns360protocol.Rr{Name: rrName, Class: dns.ClassINET, Type: rrType, Ttl: 65535, Rdata: []byte(rr[1])}
					pb.ResponseAnswerRrs = append(pb.ResponseAnswerRrs, rrPb)
					if rrType == uint32(dns.TypeCNAME) {
						rrName = GetQName(rr[1])
					}
				}

			}
		case "transaction_id":
			if i, err := strconv.Atoi(match[i]); err == nil {
				pb.DnsMessageId = uint32(i)
			}
		case "rcode":
			if t, ok := dns.StringToRcode[match[i]]; ok {
				pb.ResponseRcode = uint32(t)
			} else if t, ok := StringNumberToRcode[match[i]]; ok {
				pb.ResponseRcode = uint32(t)
			}
		}
	}

	if address := net.ParseIP(pb.ClientAddress); address == nil {
		return nil, errors.New("parse client ip address error: " + pb.ClientAddress)
	}

	if len(pb.FirstQueryName) == 0 {
		return nil, errors.New("query name is empty")
	}
	if pb.FirstType == 0 {
		pb.FirstType = uint32(dns.TypeA)
	}
	return pb, nil
}

func getDNSType(str string) uint32 {

	if t, ok := dns.StringToType[str]; ok {
		return uint32(t)
	} else if t, ok := StringNumberToType[str]; ok {
		return uint32(t)
	} else if t, ok := StringTypeNumberToType[str]; ok {
		return uint32(t)
	}
	return 0
}
