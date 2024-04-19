package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joe-ervin05/atomicbase/api"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
}

func main() {
	app := http.NewServeMux()

	api.Run(app)

	fmt.Println("Listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", app))

}
