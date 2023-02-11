package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

func main() {
	for i := 0; i < 100; i++ {
		id := ulid.Make()
		fmt.Println(strings.ToLower(id.String()))
		time.Sleep(1 * time.Second)
	}
}
