// Command basic demonstrates opening a GPIB instrument, sending an
// identification query, and reading the response.
package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/JaimeMaertz/gpib"
)

func main() {
	var dev gpib.Device

	if err := dev.Open("scope", 22); err != nil {
		log.Fatalf("open: %v", err)
	}
	defer dev.Close()

	if err := dev.Write("*IDN?"); err != nil {
		var statusErr *gpib.StatusError
		if errors.As(err, &statusErr) {
			log.Fatalf("write: %v (ibsta=0x%04x)", statusErr, statusErr.Ibsta)
		}
		log.Fatalf("write: %v", err)
	}

	resp, err := dev.Read()
	if err != nil {
		log.Fatalf("read: %v", err)
	}

	fmt.Println(resp)
}
