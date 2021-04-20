package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)


//
// BarcodeServerInterface implementation using the Alma web service
//

type AlmaServer struct {
	key string // API access key
}

// params = just the API access key
func (s *AlmaServer) Startup(params string) {
	s.key = params
}

// Dummy function (included to satisfy BarcodeServerInterface)
func (s *AlmaServer) Shutdown() {}

// Returns a BarcodeItem using the Alma database
func (s *AlmaServer) Lookup(barcode string) (*BarcodeItem) {
	API := "https://api-na.hosted.exlibrisgroup.com/almaws/v1"
	URL := fmt.Sprintf("%s/items?item_barcode=%s",API,barcode)

	client := http.Client{}
	req, err := http.NewRequest("GET",URL,nil)
	boom(err,"Unable to create HTTP request")

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("apikey %s",s.key))

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Unable to fetch Alma data for barcode "+barcode)
		return nil
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Println("Non-200 return code from Alma server!")
		log.Println("Status: ",resp.Status)
		log.Println("StatusCode: ",resp.StatusCode)
		log.Println("Header: ",resp.Header)
		log.Println("Request: ",resp.Request)
		return nil
	}

	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)

	bib_data, ok := m["bib_data"]
	if !ok {
		log.Println("Returned json data has no 'bib_data' value!")
		return nil
	}

	switch x := bib_data.(type) {
		case map[string]interface{}:
			result := BarcodeItem { Barcode: barcode }

			y, ok := x["isbn"]
			if ok { result.ISBN, _ = y.(string) }
			
			y, ok = x["author"]
			if ok { result.Author, _ = y.(string) }

			y, ok = x["title"]
			if ok { result.Title, _ = y.(string) }

			return &result

		default:
			log.Println("json 'bib_data' is not a map!")
			return nil
	}
}

// Dummy function (included to satisfy BarcodeServerInterface).
func (s *AlmaServer) Store(info *BarcodeItem) {
	log.Println("Store called on read-only Alma server!")
}
