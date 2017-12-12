package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
	"runtime"
	"math"
	"strings"
	"fmt"
	"flag"

	"github.com/golang/glog"
)

//#include <stdlib.h>
import "C"

const (
	endpointSplitter = ","
)

var counter = 0

func init() {
	flag.Parse()
}

func main() {
	// Starting HTTP server
	serverPort := os.Getenv("HTTP_SERVER_PORT")
	if serverPort != "" {
		simpleWebServer(":" + serverPort)
	}

	// Memory Load
	totalMB, err := strconv.Atoi(os.Getenv("MEM_USED_MB"))
	if err != nil {
		fmt.Printf("Error while getting MEM_USED_MB:\n %v", err)
	} else {
		totalByte := totalMB * 1024 * 1024
		memLoadGen(totalByte)
	}

	// CPU Load
	loadPercent, err := strconv.ParseFloat(os.Getenv("CPU_USED_PERCENT"), 64)
	if err != nil {
		fmt.Printf("Error while getting CPU_USED_PERCENT:\n %v", err)
	} else {
		cpuLoadGen(loadPercent)
	}

	// HTTP Load
	rps, err := strconv.ParseFloat(os.Getenv("RPS"), 64)
	if err != nil {
		glog.Errorf("Error while getting RPS: %v", err)
	} else {
		svcToTalk := os.Getenv("SVC_TO_TALK")
		svcEndpoint, err := getSvcEndpoint(svcToTalk, svcToTalk)

		if err != nil {
			glog.Errorf("Error while gettings service endpoint: %v", err)
		}

		httpLoadGen(svcEndpoint, rps)
	}

	for {
		select {
		}
	}
}

func cpuLoadGen(loadPercent float64) {
	if loadPercent <= 0 {
		return
	}

	loadPercent = math.Min(loadPercent, 1.0)

	// https://caffinc.github.io/2016/03/cpu-load-generator/
	numCPUs := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPUs)

	fmt.Printf("%d percents of CPU usage will be generated for each logical CPU of %d CPUs", int(loadPercent*100), numCPUs)

	for i := 0; i < numCPUs; i++ {
		go func() {
			for {
				if (time.Now().Nanosecond() / 1000000) % 100 == 0 { // every 100ms, sleep a while
					time.Sleep(time.Duration((1 - loadPercent) * 100) * time.Millisecond)
				}
			}
		} ()
	}
}

func memLoadGen(totalByte int) {
	if totalByte <= 0 {
		return
	}

	fmt.Printf("%d MBs memory will be allocated", totalByte/1024/1024)

	memOccupier := make([]byte, totalByte/2)

	// Use CGO to utilize the C malloc function to occupy the memory
	C.CBytes(memOccupier)
}

// Send HTTP request to the service svc
func httpLoadGen(svcEndpoint string, rps float64) {
	if rps <= 0 || svcEndpoint == "" {
		return
	}

	fmt.Printf("HTTP request will be sent to service %s with QPS %f", svcEndpoint, rps)

	requestTicker := time.NewTicker(time.Duration(float64(time.Second) / rps))

	go func() {
		for {
			select {
			case <-requestTicker.C:
				go sendRequest(svcEndpoint)
			}
		}
	}()

}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received requst at %s from %s", r.URL.Path[1:], r.RemoteAddr)

	// Send requests to other services
	svcList := os.Getenv("SVC_LIST_TO_QUERY")
	responseFromServices := make(map[string]string)
	if svcList != "" {
		svcList := strings.Split(svcList, endpointSplitter)

		for _, svc := range svcList {
			endpoint, err := getSvcEndpoint(svc, svc)
			if err != nil {
				glog.Errorf("error while getting endpoint for service %s: %v", svc, err)
				continue
			}
			responseFromServices[svc] = sendRequest(endpoint)
		}
	}

	// Process the responses from other services
	response := r.URL.Path[1:] + ":\n"
	if len(responseFromServices) > 0 {
		for svc, res := range responseFromServices {
			response += svc + " returned " + res + "\n"
		}
	}

	fmt.Printf("Response: %s", response)

	fmt.Fprintf(w, "%s", response)

}

func simpleWebServer(addr string) {
	if addr == "" {
		return
	}

	go func() {
		http.HandleFunc("/", handler)
		http.ListenAndServe(addr, nil)
	}()
}

func sendRequest(endpoint string) string {
	fmt.Printf("Sending request to %s", endpoint)
	resp, err := http.Get(endpoint)
	if err != nil {
		glog.Errorf("Error: %v", err)
		return ""
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("Error: %v", err)
		return ""
	}
	fmt.Printf("Received response: %s", string(body))
	glog.Flush()
	return string(body)
}

func getSvcEndpoint(svcName, path string) (string, error) {
	if svcName == "" {
		return "", fmt.Errorf("Error while getting service %s with path %s)\n", svcName, path)
	}
	svcEnVar := strings.Replace(strings.ToUpper(svcName), "-", "_", -1)
	svcHost := os.Getenv(svcEnVar + "_SERVICE_HOST")
	svcPort := os.Getenv(svcEnVar + "_SERVICE_PORT")
	if svcHost == "" || svcPort == "" {
		msg := fmt.Errorf("Error while getting service (%s/%s): host=%s, port=%s\n", svcName, path, svcHost, svcPort)
		glog.Error(msg)
		return "", msg
	}

	svcEndpoint := "http://" + svcHost + ":" + svcPort

	if path != "" {
		if !strings.HasPrefix(path, "/") {
			svcEndpoint += "/"
		}
		svcEndpoint += path
	}

	return svcEndpoint, nil
}