package main

import (
	"fmt"
	"github.com/30x/dispatcher/pkg/router"
)
	
func main() {

	_, err := router.ConfigFromEnv()
	if err != nil {
		fmt.Println(err)
	}
	
}
