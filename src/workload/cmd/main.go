package main

import (
	"flag"
	"log"
	"os"
	"time"
)

var (
	exitOnError  = flag.Bool("exit-on-error", false, "program should exit on I/O error")
	keepOpen     = flag.Bool("keep-open", false, "keep the file open for reading (the alternative is to loop open->read->close)")
	filePath     = flag.String("file", "", "path to file")
	readInterval = flag.Int("read-interval", 5, "time between reads")
)

func try(errMsg string, fn func() error) {
	for {
		if err := fn(); err != nil {
			if *exitOnError {
				log.Fatal(err)
			} else {
				log.Println(errMsg, err)
				time.Sleep(time.Duration(*readInterval) * time.Second)
				continue
			}
		}
		return
	}
}

func loop(okMsg, errMsg string, fn func() error) {
	for {
		if err := fn(); err != nil {
			if *exitOnError {
				log.Fatal(err)
			} else {
				log.Println(errMsg, err)
			}
		}
		log.Println(okMsg)
		time.Sleep(time.Duration(*readInterval) * time.Second)
	}
}

func main() {
	flag.Parse()

	if filePath == nil || *filePath == "" {
		log.Fatal("missing --file")
	}

	var (
		f   *os.File
		err error
		b   = make([]byte, 1)
	)

	if *keepOpen {
		try("failed to open file:", func() error {
			f, err = os.OpenFile(*filePath, os.O_RDONLY, 0644)
			return err
		})
		defer f.Close()
		log.Println("opened file", *filePath)
		loop("read file", "failed to read file:", func() error {
			_, err = f.ReadAt(b, 0)
			return err
		})
	} else {
		loop("read file", "failed to read file:", func() error {
			try("failed to open file:", func() error {
				f, err = os.OpenFile(*filePath, os.O_RDONLY, 0644)
				return err
			})
			defer f.Close()
			log.Println("opened file", *filePath)
			_, err = f.ReadAt(b, 0)
			return err
		})
	}
}
