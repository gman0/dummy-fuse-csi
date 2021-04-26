package main

import (
	"flag"
	"log"
	"os"
	"time"
)

var (
	exitOnError  = flag.Bool("exit-on-error", false, "program should exit on I/O error")
	filePath     = flag.String("file", "", "path to file")
	readInterval = flag.Int("read-interval", 5, "time between reads")
)

func main() {
	flag.Parse()

	if filePath == nil || *filePath == "" {
		log.Fatal("missing --file")
	}

	f, err := os.OpenFile(*filePath, os.O_RDONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	log.Println("opened file", *filePath)

	b := make([]byte, 1)
	for {
		log.Println("reading")

		if _, err := f.ReadAt(b, 0); err != nil {
			log.Println("read error:", err)
			if *exitOnError == true {
				os.Exit(1)
			}
		}

		time.Sleep(time.Duration(*readInterval) * time.Second)
	}
}
