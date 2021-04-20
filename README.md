# BarcodeCache

A local network cache for requests to external resources. Local cache server registration / autodetection is accomplished via [Zeroconf](http://www.zeroconf.org), with the cached data in persistent storage via one of several supported relational databases ([SQLite](https://www.sqlite.org/index.html), [MySQL](https://www.mysql.com), or [PostgreSQL](https://www.postgresql.org)).

In this example, the local server caches requests to the [Alma](https://exlibrisgroup.com/products/alma-library-services-platform/) system to translate barcode data into a book's ISBN, author, and title.

Such requests may be limited in number, either as a hard limit or through charging fees for additional requests beyond a specified amount. Furthermore, if the "external" server is down or overloaded, requests may not be serviced in a timely manner. Therefore, a local caching system that stores previously requested data can be convenient to alleviate request limits and/or provide faster response times.

The local server advertises itself on the local network via [Zeroconf](http://www.zeroconf.org), and clients send a URL request to the local server. If the local server has cached the request previously, the cached data is returned; otherwise, the "external" server is contacted, and the local server then caches the response before passing the data on to the client.

The cache itself is implemented using persistent storage via a relational database. The local server code provides a simple shim layer for interfacing with a [SQLite](https://www.sqlite.org/index.html), [MySQL](https://www.mysql.com), or [PostgreSQL](https://www.postgresql.org) database; the default mode of operation uses SQLite.

## Prerequisites

- [Go](https://golang.org)

## The server component

The server component expects incoming requests on the REST-y endpoint `/api/v1/`. At this time, only a simple barcode lookup is supported, e.g. `http://[server hostname]:[port]/api/v1/barcode/[barcode data]`. Data is returned in [JSON](https://www.json.org/json-en.html) format.

For a simple example:

```
$ cd BarcodeCache/Server
$ go run .
2021/04/20 17:11:41 Network interfaces for MacBook-Pro.local
... [preamble listing local network interfaces] ...
2021/04/20 17:11:41 Using database type 'sqlite'
2021/04/20 17:11:41 Zerconf service:
2021/04/20 17:11:41   Name: BarcodeServer
2021/04/20 17:11:41   Type: _http._tcp
2021/04/20 17:11:41   Domain: local.
2021/04/20 17:11:41   Address: :63287
```

Here, we run the local server program with no parameters; the local server will default to using a [SQLite](https://www.sqlite.org/index.html) database to cache requested data, and the "remote" server will be represented by an internal `RandomServer`. The `RandomServer` simply generates a random number and returns dummy data with that number appended for testing purposes.

The local server launches, and advertises itself using [Zeroconf](http://www.zeroconf.org) as a service called `BarcodeServer` of type `_http._tcp` in the `local` domain on port `63287` (with the latter a randomly assigned, currently unused port).

The local server can be tested using e.g. [curl](https://curl.se) via the endpoint `/api/v1/`:

```
$ curl http://localhost:63287/api/v1/
Echo: (/api/v1/) -> (127.0.0.1:63386)
```

... with the response simply echoing the request back to the user, along with printing the IP and port of the originating request.

We can also use [curl](https://curl.se) to look up a dummy barcode:

```
$ curl http://localhost:63287/api/v1/barcode/666
{"barcode":"666","isbn":"ISBN214304","author":"Author214304","title":"Title214304"}
```

Here, we assume `curl` is run on the same machine as the server, hence the use of `localhost` as the server host name. The response shows the influence of the `RandomServer` test system; dummy test data featuring a random number is generated, cached, and returned. Future lookups of the same "barcode" (`666`) should return this same data, as data for the barcode `666` is now present in the local cache and a call to the "external" `RandomServer` should not occur.

More complicated uses of the local server are possible:

```
$ go run . --help
Usage of /var/folders/9l/spnj9hpx0119g95h3bx0f14w0000gn/T/go-build1520152934/b001/exe/Server:
  -db_host string
    	Database host.
  -db_name string
    	Database name.
  -db_pass string
    	Database user password.
  -db_port string
    	Database port.
  -db_type string
    	Database type, sqlite|mysql|postgres. (default "sqlite")
  -db_user string
    	Database user name.
  -domain string
    	Set the network domain. Default should be fine. (default "local.")
  -key string
    	Alma API key.
  -name string
    	The name for the service. (default "BarcodeServer")
  -port int
    	Set the port the service is listening to (0 = use any free port).
  -type string
    	Set the server name advertised over zeroconf. (default "_http._tcp")
  -wait int
    	Timeout in seconds after which server is closed (0 = no timeout).
```

Here, we see that parameters controlling the database can be provided, along with controls for the [Zeroconf](http://www.zeroconf.org) registration.

Also present is a `key` option; this specified an [Alma](https://exlibrisgroup.com/products/alma-library-services-platform/) access key. If provided, the local server will call out to the external Alma server where a request is unable to be serviced by the local cache. The resultant data is then stored in the cache, and returned to the user.

## Example client component

The repository includes example code for a client program that automatically detects an approriate server on the local network via [Zeroconf](http://www.zeroconf.org):

```
$ go run . --help
Usage of /var/folders/9l/spnj9hpx0119g95h3bx0f14w0000gn/T/go-build1449433265/b001/exe/Client:
  -barcode string
    	Barcode to locate.
  -domain string
    	Set the search domain. For local networks, default is fine. (default "local.")
  -name string
    	The name for the service. (default "BarcodeServer")
  -service string
    	Set the service category to look for devices. (default "_http._tcp")
  -wait int
    	Duration in [s] to run discovery. (default 10)
```

The client can be passed a barcode parameter to query:

```
$ go run . -barcode 666
Service located at:  http://10.204.61.255:63774/api/v1/barcode/666
Response status: 200 OK
{"barcode":"666","isbn":"ISBN214304","author":"Author214304","title":"Title214304"}
```

By default, the client waits 10 seconds to detect the presence of a suitable local server before exit; this can be changed via the `-wait` parameter. The specifics of this detection can be controlled via the `-domain`, `-name`, and `-service` parameters.
