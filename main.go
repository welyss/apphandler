package main

import (
	"flag"
	"github.com/welyss/apphandler/service"
	"log"
	"os"
	"runtime"
)

const ()

var ()

func main() {
	var numCores = flag.Int("n", 2, "number of CPU cores to use")
	var addr = flag.String("l", ":8080", "address of listening")
	var file = flag.String("f", "config.yaml", "Config file full path")
	flag.Parse()
	runtime.GOMAXPROCS(*numCores)
	service.Run(*addr, *file)
}

func init() {
	log.SetOutput(os.Stdout)
}
