package main

import (
	"context"
	"flag"
	"log"
	"time"
	"net"
	"fmt"
	"bufio"
	"net/http"

	"github.com/grandcat/zeroconf"
)


var (
	name     = flag.String("name", "BarcodeServer", "The name for the service.")
	service  = flag.String("service", "_http._tcp", "Set the service category to look for devices.")
	domain   = flag.String("domain", "local.", "Set the search domain. For local networks, default is fine.")
	waitTime = flag.Int("wait", 10, "Duration in [s] to run discovery.")
	barcode  = flag.String("barcode", "", "Barcode to locate.")
)


func main() {
	boom := func (e error, msg string) { if e != nil { log.Fatalln(msg,"(",e,")") } }

	var svcPort int
	var svcIP net.IP
	preferIP4 := true

	flag.Parse()

	//
	// We try to read a single service entry from the channel that is passed to
	// the zerconf lookup, using a timeout
	//

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Duration(*waitTime)*time.Second)

	//
	// Create zeroconf resolver, and try to find our service of interest
	//

	resolver, err := zeroconf.NewResolver(nil)
	boom(err,"Resolver initialisation failed")

	err = resolver.Lookup(ctx, *name, *service, *domain, entries)
	boom(err,"Resolver lookup failed")

	//
	// Wait on either a resolved service result, or the timeout
	//

	select {
		case <-ctx.Done():
			log.Fatalln("Service discovery timeout")
		case e := <-entries:
			ctxCancel()
			svcPort = e.Port
			if len(e.AddrIPv4)>0 { svcIP = e.AddrIPv4[0] }
			if (preferIP4==false) && (len(e.AddrIPv6)>0) { svcIP = e.AddrIPv6[0] }
	}

	//
	// Bad server information?
	//

	if (len(svcIP)<1) || (svcPort==-1) { log.Fatalln("Unable to detect server") }

	//
	// Contact the server; if no barcode specified, just prints the server info
	// and quits.
	//

	const stem = "api/v1/"
	svcAddress := fmt.Sprintf("http://%s:%d/"+stem, svcIP, svcPort)

	if *barcode != "" {
		svcAddress += fmt.Sprintf("barcode/%s",*barcode)		
	}
	
	fmt.Println("Service located at: ", svcAddress)

	resp, err := http.Get(svcAddress)
	boom(err,"Unable to connect to service")
	
	defer resp.Body.Close()

	fmt.Println("Response status:", resp.Status)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() { fmt.Println(scanner.Text()) }
	err = scanner.Err()
	boom(err,"Problem with HTTP response scanner")
}
