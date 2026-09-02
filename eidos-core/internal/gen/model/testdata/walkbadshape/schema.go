package schema

type Thing struct {
	Stream chan int `eidos:"both,walk"`
}
