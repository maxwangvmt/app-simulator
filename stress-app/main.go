package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
	"runtime"
	"math"
	"strings"
)

//#include <stdlib.h>
import "C"

var counter = 0

func main() {
	// Starting HTTP server
	serverPort := os.Getenv("HTTP_SERVER_PORT")
	if serverPort != "" {
		simpleWebServer(":" + serverPort)
	}

	// Memory Load
	totalMB, err := strconv.Atoi(os.Getenv("MEM_USED_MB"))
	if err != nil {
		fmt.Printf("Error while getting MEM_USED_MB:\n %v\n", err)
	} else {
		totalByte := totalMB * 1024 * 1024
		memLoadGen(totalByte)
	}

	// CPU Load
	loadPercent, err := strconv.ParseFloat(os.Getenv("CPU_USED_PERCENT"), 64)
	if err != nil {
		fmt.Printf("Error while getting CPU_USED_PERCENT:\n %v\n", err)
	} else {
		cpuLoadGen(loadPercent)
	}

	// HTTP Load
	rps, err := strconv.ParseFloat(os.Getenv("RPS"), 64)
	if err != nil {
		fmt.Printf("Error while getting RPS:\n %v\n", err)
	} else {
		svcToTalk := os.Getenv("SVC_TO_TALK")
		svcEnVar := strings.Replace(strings.ToUpper(svcToTalk), "-", "_", -1)
		svcHost := os.Getenv(svcEnVar + "_SERVICE_HOST")
		svcPort := os.Getenv(svcEnVar + "_SERVICE_PORT")
		svcEndpoint := "http://" + svcHost + ":" + svcPort

		if svcToTalk == "" || svcHost == "" || svcPort == "" {
			fmt.Printf("Error while getting service (%s) endpoint: %s\n", svcToTalk, svcEndpoint)
		} else {
			httpLoadGen(svcEndpoint, rps)
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

	fmt.Printf("%d percents of CPU usage will be generated for each logical CPU of %d CPUs\n", int(loadPercent*100), numCPUs)

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
	fmt.Printf("%d MBs memory will be allocated\n", totalByte/1024/1024)

	if totalByte <= 0 {
		return
	}

	memOccupier := make([]byte, totalByte/2)

	// Use CGO to utilize the C malloc function to occupy the memory
	C.CBytes(memOccupier)
}

// Send HTTP request to the service svc
func httpLoadGen(svcEndpoint string, rps float64) {
	fmt.Printf("HTTP request will be sent to service %s with QPS %f\n", svcEndpoint, rps)

	if rps <= 0 || svcEndpoint == "" {
		return
	}
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
	fmt.Printf("[%s] Received requst at %s from %s\n", os.Getenv("HOSTNAME"), r.URL.Path[1:], r.RemoteAddr)
	fmt.Fprintf(w, "[%s] %s!", os.Getenv("HOSTNAME"), r.URL.Path[1:])
}

func simpleWebServer(addr string) {
	fmt.Printf("Starting simple server in host %s %s\n", os.Getenv("HOSTNAME"), addr)
	if addr == "" {
		return
	}

	go func() {
		http.HandleFunc("/", handler)
		http.ListenAndServe(addr, nil)
	}()
}

func sendRequest(endpoint string) {
	counter++
	endpoint = endpoint + "/" + strconv.Itoa(counter)
	resp, err := http.Get(endpoint)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	fmt.Printf("[%s] says %v\n", endpoint, string(body))
}