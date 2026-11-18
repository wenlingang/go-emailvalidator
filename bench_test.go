package emailvalidator

import "testing"

func BenchmarkValidateASCII(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := Validate("zhang.san+tag@mail.example.org"); err != nil {
			b.Fatal(err)
		}
	}
}
