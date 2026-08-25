// Utility: cetak bcrypt hash untuk membuat/mengubah akun secara manual
// (mis. INSERT langsung dari Supabase Dashboard).
// Pemakaian: go run ./cmd/hashpw <plain-text>
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "pemakaian: hashpw <plain-text>")
		os.Exit(1)
	}
	h, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(h))
}
