package emailvalidator_test

import (
	"errors"
	"fmt"

	"github.com/wenlingang/go-emailvalidator"
)

// A note glued onto the address passes net/mail but cannot be sent through
// a plain SMTP relay; the default profile rejects it with a specific reason.
func ExampleValidate() {
	err := emailvalidator.Validate("请改为此邮箱zhangsan@example.com")
	fmt.Println(errors.Is(err, emailvalidator.ErrNonASCIILocal))
	// Output: true
}

// With the IDN profile, send the Normalize result, not the original input.
func ExampleValidator_Normalize() {
	v := emailvalidator.New(emailvalidator.WithProfile(emailvalidator.IDN))
	addr, err := v.Normalize("zhangsan@例子.中国")
	fmt.Println(addr, err)
	// Output: zhangsan@xn--fsqu00a.xn--fiqs8s <nil>
}

func ExampleError() {
	err := emailvalidator.Validate("zhang san@example.com")
	fmt.Println(errors.Is(err, emailvalidator.ErrLocalChar))

	var ve *emailvalidator.Error
	if errors.As(err, &ve) {
		fmt.Println(ve.Reason, ve.Pos, ve.Input)
	}
	// Output:
	// true
	// invalid character in local part 5 zhang san@example.com
}
