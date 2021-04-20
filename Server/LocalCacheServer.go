package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

//
// Barcode item description
//

type BarcodeItem struct {
	Barcode string `json:"barcode"`
	ISBN string `json:"isbn"`
	Author string `json:"author"`
	Title string `json:"title"`
}

//
// Interfaces
//

type BarcodeServerInterface interface {
	Startup(params string)
	Shutdown()
	Lookup(barcode string) (*BarcodeItem)
	Store(info *BarcodeItem)
}

//
// Echo the incoming request information into the log
//

func echoHandler(w http.ResponseWriter, r *http.Request, internal BarcodeServerInterface, external BarcodeServerInterface) {
	txt := fmt.Sprintf("Echo: (%s) -> (%s)",r.URL.Path,r.RemoteAddr)
	w.Write( []byte(txt+"\n") )
	log.Println(txt)
}

//
// Returns the barcode item information, first trying the specified local server, then the specified external server.
//

func barcodeHandler(w http.ResponseWriter, r *http.Request, localServer BarcodeServerInterface, remoteServer BarcodeServerInterface) {
	vars := mux.Vars(r)
	barcode := vars["barcode"]
	log.Println(fmt.Sprintf("Incoming on %s : barcode \"%s\" (from %s)",r.URL.Path,barcode,r.RemoteAddr))

	w.Header().Set("Content-Type", "application/json")

	if (barcode == "") || (localServer == nil) {
		return
	}

	result := localServer.Lookup(barcode)
	
	// If local lookup failed, defer to remote server...
	if result == nil {
		log.Println( "Not found in local cache; attempting to use remote ..." )
		
		if remoteServer != nil {
			result = remoteServer.Lookup(barcode)
		} else {
			log.Println("No remote server defined!")
		}

		// Update local cache, if we get a valid result
		if result != nil { localServer.Store(result) }
	}

	// If we still lack any results, neither the local nor the remote server
	// could handle the request.
	if result != nil {
		log.Println("Result: ",result)
		err := json.NewEncoder(w).Encode(&result)
		if err != nil { log.Fatalln("Unable to write to output") }
	} else {
		log.Println("No result was located");
	}
}

//
// Parameters
//

var (
	apiKey_   = flag.String("key", "", "Alma API key.")
	domain_   = flag.String("domain", "local.", "Set the network domain. Default should be fine.")
	name_     = flag.String("name", "BarcodeServer", "The name for the service.")
	service_  = flag.String("type", "_http._tcp", "Set the server name advertised over zeroconf.")
	port_     = flag.Int("port", 0, "Set the port the service is listening to (0 = use any free port).")
	timeout_  = flag.Int("wait", 0, "Timeout in seconds after which server is closed (0 = no timeout).")

	dbType_ = flag.String("db_type", "sqlite", "Database type, sqlite|mysql|postgres.")
	dbName_ = flag.String("db_name", "", "Database name.")
	dbUser_ = flag.String("db_user", "", "Database user name.")
	dbPass_ = flag.String("db_pass", "", "Database user password.")
	dbHost_ = flag.String("db_host", "", "Database host.")
	dbPort_ = flag.String("db_port", "", "Database port.")
)

//
// Main program code
//

func main() {
	var internalServer, externalServer BarcodeServerInterface

	onShutdown := func(what string, cleanup func()) {
		log.Println( fmt.Sprintf("- Shutting down %s ...",what) )
		cleanup()
		log.Println( fmt.Sprintf("  %s shut down.",what) )
	}

	flag.Parse()

	apiKey := *apiKey_
	domain := *domain_
	name := *name_
	service := *service_
	port := *port_
	timeout := *timeout_

	dbType := *dbType_
	dbName := *dbName_
	dbUser := *dbUser_
	dbPass := *dbPass_
	dbHost := *dbHost_
	dbPort := *dbPort_

	printNetworkInterfaces()

	//
	// Boot local & remote barcode servers. We use some temp. variables
	// with same name above, so use block scoping for locals.
	//

	{
		if dbName == "" { dbName = "barcode_cache" }
		if dbHost == "" { dbHost = "localhost" }
		if dbUser == "" { dbUser = "user" }
		if dbPass == "" { dbPass = "password" }

		log.Println("Using database type '"+dbType+"'")

		var params string = ""

		switch strings.ToLower(dbType) {
			case "mysql":
				if dbPort == ""  { dbPort = "3306" }

				internalServer = &MySQLServer {}
				params = fmt.Sprintf("%s:%s@%s(%s:%s)/%s",
					dbUser, dbPass, "tcp", dbHost, dbPort, dbName)

			case "postgres":
				if dbPort == ""  { dbPort = "5432" }

				internalServer = &PostgresServer {}
				params = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
					dbHost, dbPort, dbUser, dbPass, dbName, "disable")

			case "sqlite":
				internalServer = &SQLiteServer {}
				params = fmt.Sprintf("%s.sqlite.db", dbName)

			default:
				log.Fatalln("Database type unsupported: "+dbType)
		}
		internalServer.Startup(params)
	}

	//
	// If an API key was supplied, assume we're using the Alma server as the
	// remote data source. Otherwise, use the local dummy server that returns
	// random data for storing in the local cache.
	//

	if apiKey != "" {
		externalServer = &AlmaServer {}
		externalServer.Startup(apiKey)
	} else {
		externalServer = &RandomServer {}
		externalServer.Startup("")
	}

	defer onShutdown("internal barcode server", func() {internalServer.Shutdown()} )
	defer onShutdown("external barcode server", func() {externalServer.Shutdown()} )

	// Catch user interrupt signal on channel for clean shutdown

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	// Set up web server. Ideally, we'd drain requests before shutdown.

	const apiPrefix = "/api/v1/"

	handler := mux.NewRouter()

	handler.HandleFunc( "/", func(w http.ResponseWriter, r *http.Request) {
		echoHandler(w,r,internalServer,externalServer)
	});

	handler.HandleFunc( apiPrefix, func(w http.ResponseWriter, r *http.Request) {
		echoHandler(w,r,internalServer,externalServer)
	});

	handler.HandleFunc( apiPrefix+"barcode/{barcode}", func(w http.ResponseWriter, r *http.Request) {
		barcodeHandler(w,r,internalServer,externalServer)
	});

	// Using an explicit Listener provides more control over the specifics,
	// e.g. tcp4/6 and letting the system select a currently unused port.

	apiServer := http.Server {
		Handler: handler,
	}
	var listener net.Listener = nil

	if port<1 {
		var err error

		listener, err = net.Listen("tcp4", fmt.Sprintf(":%d",port)) // ":0" -> all addresses, use any free port
		boom(err, "net.Listen failed")
		
		defer onShutdown("listener", func() {listener.Close()} )

		// We need the port number for zeroconf registration - extract from
		// new address string, as port may have be assigned by system.

		_, portStr, err := net.SplitHostPort(listener.Addr().String())
		boom(err, "net.SplitHostPort failed")

		port, err = strconv.Atoi(portStr)
		boom(err, "strconv.Atoi failed")
	}

	apiServer.Addr = fmt.Sprintf(":%d",port) // use (re-)assigned port

	// Run web server in a separate goroutine so it doesn't block our progress

	go func() {
		var err error

		if listener == nil {
			err = apiServer.ListenAndServe()
		} else {
			err = apiServer.Serve(listener)
		}

		switch err {
			case nil:
			case http.ErrServerClosed:
				log.Println("Caught ErrServerClosed")
			default:
				panic(err)
		}
	}()

	defer onShutdown("API server", func() {apiServer.Shutdown(context.Background())} )

	// Launch Zeroconf server to adversize the service

	zcServer := ZeroconfServer {}
	err := zcServer.Startup(name,port,nil)
	boom(err, "ZerconfServer startup failed")
	defer onShutdown("ZeroconfServer", func() {zcServer.Shutdown()} )

	log.Println("Zerconf service:")
	log.Println("  Name:", name)
	log.Println("  Type:", service)
	log.Println("  Domain:", domain)
	log.Println("  Address:", apiServer.Addr)

	// Timeout channel, if needed

	tc := make(<-chan time.Time);
	if timeout > 0 {
		tc = time.After(time.Second * time.Duration(timeout))
	}

	// Wait on user interruption or timeout

	select {
		case <-sig: // user interruption
		case <-tc: // timeout
	}

	log.Println("Shutting down.")
}
