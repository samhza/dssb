package main

import (
	"log"
	"net/http"

	"go.samhza.com/dssb/dashboard"
)

func main() {
	dashboard := dashboard.New()
	err := dashboard.StartServer()
	if err != nil {
		log.Fatalln("starting server:", err)
	}
	err = http.ListenAndServe(":8080", dashboard)
	if err != nil {
		log.Fatalln(err)
	}
}
