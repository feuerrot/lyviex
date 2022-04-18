package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.bug.st/serial"
)

var (
	hdr    = []byte{0x16, 0x11, 0x0B}
	pmstat = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sensor",
		ConstLabels: map[string]string{
			"sensor": "PM1006",
			"type":   "PM",
		},
	}, []string{"value"})

	pms1_0 prometheus.Gauge
	pms2_5 prometheus.Gauge
	pms10  prometheus.Gauge
)

func main() {
	if len(os.Args) < 2 {
		ports, err := serial.GetPortsList()
		if err != nil {
			log.Fatal(err)
		}
		if len(ports) == 0 {
			log.Fatal("No serial ports found!")
		}
		for _, port := range ports {
			log.Printf("Found port: %v\n", port)
		}

		log.Printf("Usage: %s [serial port]", os.Args[0])
		os.Exit(1)
	}

	mode := &serial.Mode{
		BaudRate: 9600,
	}
	port, err := serial.Open(os.Args[1], mode)
	if err != nil {
		log.Fatal(err)
	}

	pms1_0, _ = pmstat.GetMetricWithLabelValues("1")
	pms2_5, _ = pmstat.GetMetricWithLabelValues("2.5")
	pms10, _ = pmstat.GetMetricWithLabelValues("10")

	go func() {

		ctr := 0
		buff := make([]byte, 1)
		row := make([]byte, 20)
		for {
			n, err := port.Read(buff)
			if err != nil {
				log.Fatal(err)
				break
			}
			if n == 0 {
				log.Println("\nEOF")
				break
			}
			if ctr < 3 && buff[0] != hdr[ctr] {
				ctr = 0
				continue
			}

			row[ctr] = buff[0]
			ctr += 1

			if ctr == 20 {
				parseBuf(row)
				ctr = 0
			}
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":33141", nil)
}

func parseBuf(b []byte) {
	var sum byte
	var cs string
	for _, e := range b {
		sum += e
	}
	if sum == 0x00 {
		cs = "Checksum OK"
	} else {
		cs = fmt.Sprintf("Checksum ERR: %X\n", sum)
	}

	log.Printf("%X (%s)", b, cs)

	pm2_5 := int(b[5])*256 + int(b[6])
	pm1_0 := int(b[9])*256 + int(b[10])
	pm10 := int(b[13])*256 + int(b[14])

	pms1_0.Set(float64(pm1_0))
	pms2_5.Set(float64(pm2_5))
	pms10.Set(float64(pm10))

	log.Printf("PM2.5: %d\tPM1.0: %d\tPM10: %d\n", pm2_5, pm1_0, pm10)
}
