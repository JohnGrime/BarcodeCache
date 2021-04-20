package main

import (
	"fmt"
//	"hash/adler32"
	"log"
	"math/rand"
	"net"
	"os"
)

// For when things go badly wrong
func boom (e error, msg string) {
	if e != nil {
		log.Fatalln(msg,"(",e,")")
	}
}

//
// BarcodeServerInterface implementation using random data
//

type RandomServer struct {}

// Dummy function (included to satisfy BarcodeServerInterface)
func (s *RandomServer) Startup(_ string) {}

// Dummy function (included to satisfy BarcodeServerInterface)
func (s *RandomServer) Shutdown() {}

// Returns a BarcodeItem structure with the given barcode and random data in other fields
func (s *RandomServer) Lookup(barcode string) (*BarcodeItem) {
	r := rand.Intn(1000000)
	return &BarcodeItem {
		Barcode: barcode,
		ISBN: fmt.Sprintf("ISBN%d",r),
		Author: fmt.Sprintf("Author%d",r),
		Title: fmt.Sprintf("Title%d",r),
	}
}

// Dummy function (included to satisfy BarcodeServerInterface)
func (s *RandomServer) Store(info *BarcodeItem) {
	log.Println("Store called on read-only random server!")
}

//
// Print information about the local machine's network interfaces
//

func printNetworkInterfaces() {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic("net.Interfaces()")
	}

	if len(ifaces)<1 {
		log.Println("No network interfaces found.")
		return
	}

	hostname, _ := os.Hostname()
	log.Println( "Network interfaces for " + hostname )

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil { panic("iface.Addrs()") }
		if len(addrs) < 1 { continue }
		log.Println("-",iface.Name,iface.HardwareAddr)
		for _, addr := range addrs {
			switch v := addr.(type) {
				case *net.IPNet:
					str := fmt.Sprintf("IPNet: IP=%s, mask=%s, network=%s, string=%s", v.IP, v.Mask, v.Network(), v.String())
					log.Println(" ", str)
				case *net.IPAddr:
					str := fmt.Sprintf("IPAddr: IP=%s, zone=%s, network=%s, string=%s", v.IP, v.Zone, v.Network(), v.String())
					log.Println(" ", str)
				default:
					log.Println("<unknown>")
			}
		}
	}
}
