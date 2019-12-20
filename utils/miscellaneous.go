package utils

import (
	"net"
	"net/http"
	"os"
	"strings"
)

func GetIp(r *http.Request) string {
	sourceIp := r.Header.Get("X-FORWARDED-FOR")
	if sourceIp == "" {
		sourceIp, _, _ = net.SplitHostPort(r.RemoteAddr)
	}

	return sourceIp
}

// GetEnv Obtains the environment key or returns the default value
func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func GetLocalIP() string {

	addrs, _ := net.InterfaceAddrs()

	currentIP := ""

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			currentIP = ipnet.IP.String()
			break
		} else {
			currentIP = ipnet.IP.String()
		}
	}

	return currentIP

}

func GetMacAddress() string {

	currentIP := GetLocalIP()

	interfaces, _ := net.Interfaces()
	for _, interf := range interfaces {

		if addrs, err := interf.Addrs(); err == nil {
			for _, addr := range addrs {

				// only interested in the name with current IP address
				if strings.Contains(addr.String(), currentIP) {
					return interf.HardwareAddr.String()
				}
			}
		}
	}

	return ""

}
