package main
import (
	"fmt"
)
func f() func() { return func() {} }
func main() {
	a := f()
	b := f()
	fmt.Printf("Direct: %v\n", a == b)
	fmt.Printf("Sprintf: %v\n", fmt.Sprintf("%p", a) == fmt.Sprintf("%p", b))
}
