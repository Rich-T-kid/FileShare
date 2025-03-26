package main

import (
	"bytes"
	"fmt"
)

type Data struct {
	ID    int32
	Value float64
	Text  string
}

func Join(s []string, sep string) string {
	//b := make([]byte, 1)
	var bb bytes.Buffer
	//bb.
	for i := range s {
		str := s[i]
		//b = append(b, []byte(str)...)
		//b = append(b, []byte(sep)...)
		bb.WriteString(str)
		bb.WriteString(sep)
	}
	//b = b[:len(b)-len([]byte(sep))]
	//return string(b)
	res := bb.String()
	return res[:len(res)-len(sep)]

}
func main() {
	names := []string{"tomato", "salad", "avacado", "ranch"}
	res := Join(names, "-")
	fmt.Println(res)
}
