package main

import (
	"fmt"
	"strings"
)

func main() {
	// Test the POS parsing logic
	pos := "助詞,係助詞,*,*"
	tokenPOS := strings.SplitN(pos, ",", 2)[0]
	fmt.Printf("Original POS: %s\n", pos)
	fmt.Printf("Token POS: %s\n", tokenPOS)
	fmt.Printf("Is particle?: %t\n", tokenPOS == "助詞")
}
