package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joe-ervin05/atomicbase/api"
	"github.com/joe-ervin05/atomicbase/config"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
}

func main() {
	app := http.NewServeMux()

	api.Run(app)

	fmt.Printf("Listening on port %s\n", config.Cfg.Port)
	log.Fatal(http.ListenAndServe(config.Cfg.Port, app))

}
