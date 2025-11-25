package main

import (
	"log"
	"log/syslog"
	"testing"
)

func TestRegexp6(t *testing.T) {
	sysLog, err := syslog.Dial("udp", "localhost:5141", syslog.LOG_INFO, "golang")
	if err != nil {

		log.Fatal(err)
	}

	str := "52.72.72.72|53|160.66.45.90|48152|17539|1Kkk-8tm1-kv-cnP.yoKP4ncyn.com|AAAA|(CNAME)_1Kkk-8tm1-kv-cnP.yoKP4ncyn.com.wAyXA.com|20230320123004.002|0|r"
	str = "56.56.56.56|53|160.66.227.8|24553|13511|J7.aLJ.JY.vvY.4n-syy8.s81s|PTR||20230320123004.001|2|r"
	str = "52.72.72.72|53|160.66.23.163|30342|31035|8499165.com|A|(CNAME)_1Kkk-UkX-ka7-cnP.yoKP4ncyn.com.c.cyn9wcJ.com;(CNAME)_9cynk.1KkkyP4n-X7.gAkQ.c.cyn9wca.com;(A)_27.199.158.226;(A)_35.50.63.134;(A)_27.199.158.247;(A)_35.50.63.136;(A)_84.212.13.131;(A)_35.50.63.133;(A)_84.212.13.134;(A)_84.212.13.28;(A)_84.212.13.29;(A)_84.212.13.135;(A)_35.50.63.138;(A)_35.50.63.135|20230320123004.002|0|r"
	_ = sysLog.Emerg(str)

}
