package bytesize

import "fmt"

func ExampleBytes() {
	fmt.Println(B * 512)
	fmt.Println(Bytes(-1024))
	fmt.Println(KiB + B*500)
	fmt.Println(5 * MiB)
	// Output:
	// 512B
	// -1.00KiB
	// 1.49KiB
	// 5.00MiB
}
